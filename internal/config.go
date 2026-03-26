package internal

import (
	"fmt"
	"reflect"
	"strconv"
)

// SetDefaultByTag 根据结构体字段的tag设置默认值，包括嵌套对象和指针
func SetDefaultByTag(obj interface{}) {
	// 获取对象的反射值
	v := reflect.ValueOf(obj)
	// 确保对象是可设置的（非指针或指针指向的值）
	if v.Kind() != reflect.Ptr || !v.Elem().CanSet() {
		panic("SetDefaultByTag requires a pointer to a struct")
	}
	v = v.Elem()

	// 递归遍历结构体的所有字段
	setDefaults(v)
}

// setDefaults 递归设置默认值
func setDefaults(v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() {
			continue // 如果字段不可设置，则跳过
		}
		if field.Kind() == reflect.Ptr {
			// 如果字段是指针类型，需要特别处理
			// 尝试创建一个新的实例
			ptrValue := reflect.New(field.Type().Elem())
			if ptrValue.Type() == field.Type() {
				// 递归设置指针指向的值
				setDefaults(ptrValue.Elem())
				// 将指针指向新的实例
				field.Set(ptrValue)
			}
			continue
		} else if field.Kind() == reflect.Struct {
			// 如果是结构体，则递归调用setDefaults
			setDefaults(field)
			continue
		}

		// 获取字段的tag
		tag := v.Type().Field(i).Tag
		defaultValue := tag.Get("default") // 从tag中获取默认值

		// 仅在字段为零值时才设置默认值，避免覆盖用户传入的配置
		if defaultValue != "" && field.IsZero() {
			switch field.Kind() {
			case reflect.String:
				field.SetString(defaultValue)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				val, err := strconv.ParseInt(defaultValue, 10, 64)
				if err == nil {
					field.SetInt(val)
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				val, err := strconv.ParseUint(defaultValue, 10, 64)
				if err == nil {
					field.SetUint(val)
				}
			case reflect.Float32, reflect.Float64:
				val, err := strconv.ParseFloat(defaultValue, 64)
				if err == nil {
					field.SetFloat(val)
				}
			case reflect.Bool:
				val, err := strconv.ParseBool(defaultValue)
				if err == nil {
					field.SetBool(val)
				}
			default:
				fmt.Printf("Unsupported type for field %s\n", v.Type().Field(i).Name)
			}
		}
	}
}
