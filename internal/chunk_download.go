package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/imroc/req/v3"
	"io"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	runningMu  sync.RWMutex
	runningMap = make(map[*ChunkDownload]bool)
)

func isShutdown() bool {
	return GlobalContext().Err() != nil
}

// ProgressFunc 下载进度回调，签名与 pan.ProgressCallback 一致
type ProgressFunc func(fileName string, operated, totalSize int64, percent, speed float64, done bool)

type ChunkDownload struct {
	url             string
	client          *req.Client
	concurrency     int
	maxRetry        int
	maxThread       int
	output          io.Writer
	filename        string
	outputDirectory string
	totalBytes      int64
	chunkSize       int64
	tempRootDir     string
	tempDir         string
	semaphore       chan struct{}
	taskCh          chan *downloadTask
	doneCh          chan struct{}
	wgDoneCh        chan struct{}
	errCh           chan error
	wg              sync.WaitGroup
	taskMap         map[int]*downloadTask
	taskNotifyCh    chan *downloadTask
	mu              sync.Mutex
	lastIndex       int
	pw              *progressWriter
	progressFunc    ProgressFunc
}

func NewChunkDownload(url string, client *req.Client) *ChunkDownload {
	return &ChunkDownload{
		url:    url,
		client: client,
	}
}

// SetProgressFunc 设置下载进度回调
func (pd *ChunkDownload) SetProgressFunc(fn ProgressFunc) *ChunkDownload {
	pd.progressFunc = fn
	return pd
}

func (pd *ChunkDownload) completeTask(task *downloadTask) {
	pd.mu.Lock()
	pd.taskMap[task.index] = task
	pd.mu.Unlock()
	go func() {
		select {
		case pd.taskNotifyCh <- task:
		case <-pd.doneCh:
		}
	}()
}

func (pd *ChunkDownload) popTask(index int) (*downloadTask, bool) {
	pd.mu.Lock()
	if task, ok := pd.taskMap[index]; ok {
		delete(pd.taskMap, index)
		pd.mu.Unlock()
		return task, true
	}
	pd.mu.Unlock()
	for {
		select {
		case task := <-pd.taskNotifyCh:
			if task.index == index {
				pd.mu.Lock()
				delete(pd.taskMap, index)
				pd.mu.Unlock()
				return task, true
			}
			// put non-matching task back into taskMap
			pd.mu.Lock()
			pd.taskMap[task.index] = task
			pd.mu.Unlock()
		case <-pd.doneCh:
			return nil, false
		case <-GlobalContext().Done():
			return nil, false
		}
	}
}

func (pd *ChunkDownload) ensure() error {
	// 若未设置，则仅单线程下载就好
	if pd.concurrency <= 0 {
		pd.concurrency = 1
	}
	if pd.chunkSize <= 0 {
		pd.chunkSize = 1024 * 1024 * 10 // 10MB
	}
	if pd.tempRootDir == "" {
		pd.tempRootDir = os.TempDir()
	}
	if pd.maxRetry <= 0 {
		pd.maxRetry = 3
	}
	if pd.maxThread <= 0 {
		pd.maxThread = 50
	}
	pd.semaphore = make(chan struct{}, pd.maxThread)

	fullPath, err := filepath.Abs(pd.filename)
	if err != nil {
		return err
	}
	pd.tempDir = filepath.Join(pd.tempRootDir, Md5HashStr(fullPath))

	err = os.MkdirAll(pd.tempDir, os.ModePerm)
	if err != nil {
		return err
	}

	pd.taskCh = make(chan *downloadTask)
	pd.doneCh = make(chan struct{})
	pd.wgDoneCh = make(chan struct{})
	pd.errCh = make(chan error)
	pd.taskMap = make(map[int]*downloadTask)
	pd.taskNotifyCh = make(chan *downloadTask)

	pd.pw = &progressWriter{
		totalSize:    pd.totalBytes,
		fileName:     pd.filename,
		startTime:    time.Now(),
		progressFunc: pd.progressFunc,
	}
	return nil
}

func (pd *ChunkDownload) SetChunkSize(chunkSize int64) *ChunkDownload {
	pd.chunkSize = chunkSize
	return pd
}

func (pd *ChunkDownload) SetTempRootDir(tempRootDir string) *ChunkDownload {
	pd.tempRootDir = tempRootDir
	return pd
}

func (pd *ChunkDownload) SetConcurrency(concurrency int) *ChunkDownload {
	pd.concurrency = concurrency
	return pd
}

func (pd *ChunkDownload) SetOutput(output io.Writer) *ChunkDownload {
	if output != nil {
		pd.output = output
	}
	return pd
}

func (pd *ChunkDownload) SetOutputFile(filename string) *ChunkDownload {
	pd.filename = filename
	return pd
}
func (pd *ChunkDownload) SetFileSize(totalBytes int64) *ChunkDownload {
	pd.totalBytes = totalBytes
	return pd
}

func (pd *ChunkDownload) SetOutputDirectory(outputDirectory string) *ChunkDownload {
	pd.outputDirectory = outputDirectory
	return pd
}

func (pd *ChunkDownload) SetMaxRetry(n int) *ChunkDownload {
	pd.maxRetry = n
	return pd
}

func (pd *ChunkDownload) SetMaxThread(n int) *ChunkDownload {
	pd.maxThread = n
	return pd
}

func getRangeTempFile(rangeStart, rangeEnd int64, workerDir string) string {
	return filepath.Join(workerDir, fmt.Sprintf("temp-%d-%d", rangeStart, rangeEnd))
}

func getRangeStartEnd(filename string) (int64, int64) {
	// 获取文件名部分，去掉路径
	baseName := filepath.Base(filename)

	// 去掉前缀 "temp-"
	parts := strings.Split(baseName, "-")
	if len(parts) != 3 || parts[0] != "temp" {
		return 0, 0 // 或者返回错误信息
	}

	// 解析 rangeStart 和 rangeEnd
	rangeStart, err1 := strconv.ParseInt(parts[1], 10, 64)
	rangeEnd, err2 := strconv.ParseInt(parts[2], 10, 64)

	if err1 != nil || err2 != nil {
		return 0, 0 // 或者返回错误信息
	}

	return rangeStart, rangeEnd
}

type downloadTask struct {
	index                           int
	rangeStart, rangeEnd, totalSize int64
	tempFilename                    string
	completed                       bool
	tempFile                        *os.File
	retry                           int
}

func (pd *ChunkDownload) handleTask(t *downloadTask, ctx ...context.Context) {
	pd.wg.Add(1)
	defer pd.wg.Done()
	if isShutdown() {
		return
	}
	if t.completed {
		pd.pw.updateDownloaded(t.totalSize)
		pd.completeTask(t)
		return
	}

	file, eo := os.OpenFile(t.tempFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if eo != nil {
		pd.errCh <- eo
		return
	}
	cpr := &chunkProgressWriter{
		startTime:    time.Now(),
		fileName:     t.tempFilename,
		progressFunc: pd.progressFunc,
	}
	r := pd.client.R().
		SetHeader("Range", fmt.Sprintf("bytes=%d-%d", t.rangeStart, t.rangeEnd)).
		SetOutput(file).
		SetDownloadCallback(cpr.downloadCallback)
	if len(ctx) > 0 && ctx[0] != nil {
		r = r.SetContext(ctx[0])
	}
	resp, er := r.Get(pd.url)
	if er != nil {
		file.Close()
		go pd.retry(t, er)
		return
	}
	if resp.IsErrorState() {
		file.Close()
		go pd.retry(t, fmt.Errorf("request error: %s", resp.String()))
		return
	}
	t.tempFile = file
	pd.pw.updateDownloading(t.totalSize)
	pd.completeTask(t)
}

func (pd *ChunkDownload) retry(t *downloadTask, err error) {
	if t.retry < pd.maxRetry {
		GetLogger().Error("task error", "task", t.tempFilename, "error", err)
		t.retry += 1
		pd.taskCh <- t
	} else {
		pd.errCh <- err
	}
}

func (pd *ChunkDownload) startWorker(ctx ...context.Context) {
	gCtx := GlobalContext()
	var clientDone <-chan struct{}
	if len(ctx) > 0 && ctx[0] != nil {
		clientDone = ctx[0].Done()
	}
	for {
		select {
		case t := <-pd.taskCh:
			pd.semaphore <- struct{}{}
			pd.handleTask(t, ctx...)
			<-pd.semaphore
		case <-pd.doneCh:
			return
		case <-gCtx.Done():
			return
		case <-clientDone:
			return
		}
	}
}

func (pd *ChunkDownload) mergeFile() {
	defer pd.wg.Done()
	file, err := pd.getOutputFile()
	if err != nil {
		pd.errCh <- err
		return
	}
	if closer, ok := file.(io.Closer); ok {
		defer closer.Close()
	}
	for i := 0; ; i++ {
		if isShutdown() {
			return
		}
		task, ok := pd.popTask(i)
		if !ok {
			// shutdown triggered, abort merge
			return
		}
		tempFile, eo := os.Open(task.tempFilename)
		if eo != nil {
			pd.errCh <- eo
			return
		}
		_, eo = io.Copy(file, tempFile)
		tempFile.Close()
		if eo != nil {
			pd.errCh <- eo
			return
		}
		_ = os.Remove(task.tempFilename)
		if i >= pd.lastIndex {
			break
		}
	}

	err = os.RemoveAll(pd.tempDir)
	if err != nil {
		pd.errCh <- err
	}
}

func (pd *ChunkDownload) Do(ctx ...context.Context) error {
	if isShutdown() {
		return errors.New("service is shutdown")
	}

	ShutdownWg().Add(1)
	defer ShutdownWg().Done()

	err := pd.ensure()
	if err != nil {
		return err
	}
	for i := 0; i < pd.concurrency; i++ {
		go pd.startWorker(ctx...)
	}
	if pd.totalBytes == 0 {
		resp := pd.client.Head(pd.url).Do(ctx...)
		if resp.Err != nil {
			return resp.Err
		}
		if resp.ContentLength <= 0 {
			return fmt.Errorf("bad content length: %d", resp.ContentLength)
		}
		pd.totalBytes = resp.ContentLength
	}

	runningMu.Lock()
	runningMap[pd] = true
	runningMu.Unlock()

	pd.wg.Add(1)
	go pd.mergeFile()
	go func() {
		pd.wg.Wait()
		close(pd.wgDoneCh)
	}()

	go pd.calTask()

	select {
	case <-pd.wgDoneCh:
		close(pd.doneCh)
		runningMu.Lock()
		delete(runningMap, pd)
		runningMu.Unlock()
	case err := <-pd.errCh:
		close(pd.doneCh)
		runningMu.Lock()
		delete(runningMap, pd)
		runningMu.Unlock()
		return err
	}
	return nil
}

type Range struct {
	start     int64
	end       int64
	completed bool
	fileName  string
}

func (pd *ChunkDownload) calTask() {
	ranges, err := pd.CalRange()
	if err != nil {
		pd.errCh <- err
		return
	}
	pd.lastIndex = len(ranges) - 1
	for i, r := range ranges {
		task := &downloadTask{
			tempFilename: r.fileName,
			index:        i,
			rangeStart:   r.start,
			rangeEnd:     r.end,
			completed:    r.completed,
			totalSize:    r.end - r.start + 1,
		}
		select {
		case pd.taskCh <- task:
		case <-pd.doneCh:
			return
		case <-GlobalContext().Done():
			return
		}
	}
}

func (pd *ChunkDownload) CalRange() ([]Range, error) {
	dir, err := os.ReadDir(pd.tempDir)
	if err != nil {
		return nil, err
	}
	rangeMap := make(map[int64]Range)
	keys := make([]int64, 0)
	for _, entry := range dir {
		name := entry.Name()
		s, e := getRangeStartEnd(name)
		if e-s > 0 {
			fileInfo, err := entry.Info()
			if err != nil {
				return nil, err
			}
			// 文件大小不一致，执行清理
			if fileInfo.Size() != e-s+1 {
				_ = os.Remove(pd.tempDir + "/" + name)
			} else {
				rangeMap[s] = Range{start: s, end: e, completed: true, fileName: filepath.Join(pd.tempDir, name)}
				keys = append(keys, s)
			}
		} else {
			_ = os.Remove(pd.tempDir + "/" + name)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	var start int64 = 0
	// 由于合并后会移除临时文件，所以判断文件是否存在，用其大小作为开始下载的分片
	if pd.output == nil {
		fileInfo, _ := IsExistFile(calFileName(pd.filename, pd.outputDirectory, pd.url))
		if fileInfo != nil {
			start = fileInfo.Size()
		}
	}
	pd.pw.updateDownloaded(start)
	ranges := make([]Range, 0)
	for _, key := range keys {
		r := rangeMap[key]
		if r.start > start {
			maxEnd := r.start - 1
			start, ranges = pd.addRange(start, maxEnd, ranges)
		}
		ranges = append(ranges, r)
		start = r.end + 1
	}

	if start < pd.totalBytes {
		start, ranges = pd.addRange(start, pd.totalBytes-1, ranges)
	}

	return ranges, nil
}

func (pd *ChunkDownload) addRange(start int64, maxEnd int64, ranges []Range) (int64, []Range) {
	for {
		end := start + (pd.chunkSize - 1)
		if end > maxEnd {
			end = maxEnd
		}
		if end != start {
			ranges = append(ranges, Range{start: start, end: end, completed: false, fileName: getRangeTempFile(start, end, pd.tempDir)})
			start = end + 1
		}
		if end >= maxEnd {
			break
		}
	}
	return start, ranges
}

func (pd *ChunkDownload) getOutputFile() (io.Writer, error) {
	outputFile := pd.output
	if outputFile != nil {
		return outputFile, nil
	}
	pd.filename = calFileName(pd.filename, pd.outputDirectory, pd.url)
	err := os.MkdirAll(filepath.Dir(pd.filename), os.ModePerm)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(pd.filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModePerm)
}

func calFileName(filename, outputDirectory, url string) string {
	retFileName := filename
	if retFileName == "" {
		u, err := urlpkg.Parse(url)
		if err != nil {
			panic(err)
		}
		paths := strings.Split(u.Path, "/")
		for i := len(paths) - 1; i > 0; i-- {
			if paths[i] != "" {
				retFileName = paths[i]
				break
			}
		}
		if retFileName == "" {
			retFileName = "download"
		}
	}
	if outputDirectory != "" && !filepath.IsAbs(retFileName) {
		retFileName = filepath.Join(outputDirectory, retFileName)
	}
	return retFileName
}

type progressWriter struct {
	downloaded     int64
	thisDownloaded int64
	totalSize      int64
	fileName       string
	startTime      time.Time
	m              sync.Mutex
	progressFunc   ProgressFunc
}

func (p *progressWriter) updateDownloading(downloaded int64) {
	p.m.Lock()
	defer p.m.Unlock()
	p.thisDownloaded += downloaded
	p.downloaded += downloaded
	p.log()
}

func (p *progressWriter) updateDownloaded(downloaded int64) {
	p.m.Lock()
	defer p.m.Unlock()
	p.downloaded += downloaded
	p.log()
}

func (p *progressWriter) log() {
	LogProgress("downloading", p.fileName, p.startTime, p.thisDownloaded, p.downloaded, p.totalSize, true)
	if p.progressFunc != nil {
		elapsed := time.Since(p.startTime).Seconds()
		var speed float64
		if elapsed > 0 {
			speed = float64(p.thisDownloaded) / 1024 / elapsed
		}
		percent := float64(p.downloaded) / float64(p.totalSize) * 100
		done := p.downloaded >= p.totalSize
		p.progressFunc(p.fileName, p.downloaded, p.totalSize, percent, speed, done)
	}
}

type chunkProgressWriter struct {
	downloaded   int64
	totalSize    int64
	startTime    time.Time
	fileName     string
	progressFunc ProgressFunc
}

func (c *chunkProgressWriter) log() {
	LogProgress("downloading", c.fileName, c.startTime, c.downloaded, c.downloaded, c.totalSize, false)
	if c.progressFunc != nil {
		elapsed := time.Since(c.startTime).Seconds()
		var speed float64
		if elapsed > 0 {
			speed = float64(c.downloaded) / 1024 / elapsed
		}
		percent := float64(c.downloaded) / float64(c.totalSize) * 100
		done := c.downloaded >= c.totalSize
		c.progressFunc(c.fileName, c.downloaded, c.totalSize, percent, speed, done)
	}
}

func (c *chunkProgressWriter) downloadCallback(info req.DownloadInfo) {
	if info.Response.Response != nil {
		c.totalSize = info.Response.ContentLength
		c.downloaded = info.DownloadedSize
		c.log()
	}
}
