package fieldmask_utils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
)

func ProtoToStruct(
	fm *types.FieldMask,
	src, dst interface{},
	opts ...interface{},
) error {
	filter, err := MaskFromProtoFieldMask(fm, opts...)
	if err != nil {
		return err
	}

	return StructToStruct(filter, src, dst)
}

// StructToStruct copies `src` struct to `dst` struct using the given FieldFilter.
// Only the fields where FieldFilter returns true will be copied to `dst`.
// `src` and `dst` must be coherent in terms of the field names, but it is not required for them to be of the same type.
func StructToStruct(filter FieldFilter, src, dst interface{}) error {
	srcVal := indirect(reflect.ValueOf(src))
	dstVal := indirect(reflect.ValueOf(dst))
	srcFields := getFieldMappingFromTags(srcVal, false)
	dstFields := getFieldMappingFromTags(dstVal, true)

	for i := 0; i < srcVal.NumField(); i++ {
		f := srcVal.Field(i)
		fieldName := srcVal.Type().Field(i).Name
		if _, ok := srcFields[fieldName]; !ok {
			continue
		}

		srcFieldName := srcFields[fieldName]

		subFilter, ok := filter.Filter(srcFieldName)
		if !ok {
			// Skip this field.
			continue
		}
		if !f.CanSet() {
			return errors.Errorf("can't set a value on a field %s", fieldName)
		}

		if _, ok := dstFields[srcFieldName]; !ok {
			return errors.Errorf("target field %s is not present in dst struct", srcFieldName)
		}

		srcField, err := getField(src, fieldName)
		if err != nil {
			return errors.Wrapf(err, "failed to get the field %s from %T", fieldName, src)
		}
		dstField, err := getField(dst, dstFields[srcFieldName])
		if err != nil {
			return errors.Wrapf(err, "failed to get the field %s from %T", fieldName, dst)
		}

		dstFieldType := dstField.Type()

		switch dstFieldType.Kind() {
		case reflect.Interface:
			if srcField.IsNil() {
				dstField.Set(reflect.Zero(dstFieldType))
				continue
			}
			if !srcField.Type().Implements(dstFieldType) {
				return errors.Errorf("src %T does not implement dst %T",
					srcField.Interface(), dstField.Interface())
			}

			v := reflect.New(srcField.Elem().Elem().Type())
			if err := StructToStruct(subFilter, srcField.Interface(), v.Interface()); err != nil {
				return err
			}
			dstField.Set(v)

		case reflect.Ptr:
			switch srcField.Type().Kind() {
			case reflect.Ptr, reflect.Interface:
				if srcField.IsNil() {
					dstField.Set(reflect.Zero(dstFieldType))
					continue
				}

				v := reflect.New(dstFieldType.Elem())
				if err := StructToStruct(subFilter, srcField.Interface(), v.Interface()); err != nil {
					return err
				}
				dstField.Set(v)

			default:
				v := reflect.New(dstFieldType.Elem())
				v.Elem().Set(srcField)
				dstField.Set(v)
			}

		case reflect.Array, reflect.Slice:
			// Check if it is an array of values (non-pointers).
			if dstFieldType.Elem().Kind() != reflect.Ptr {
				// Handle this array/slice as a regular non-nested data structure: copy it entirely to dst.
				dstField.Set(srcField)
				continue
			}
			v := reflect.New(dstFieldType).Elem()
			// Iterate over items of the slice/array.
			for i := 0; i < srcField.Len(); i++ {
				subValue := srcField.Index(i)
				newDst := reflect.New(dstFieldType.Elem().Elem())
				if err := StructToStruct(subFilter, subValue.Interface(), newDst.Interface()); err != nil {
					return err
				}
				v.Set(reflect.Append(v, newDst))
			}
			dstField.Set(v)

		default:
			// For primitive data types just copy them entirely.
			dstField.Set(srcField)
		}
	}
	return nil
}

func getFieldMappingFromTags(val reflect.Value, reverse bool) map[string]string {
	fields := map[string]string{}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		tag := field.Tag

		var spec string

		switch {
		case tag.Get("protobuf") != "":
			spec = tag.Get("protobuf")

		case tag.Get("protobuf_oneof") != "":
			spec = "name=" + tag.Get("protobuf_oneof")

		case tag.Get("json") != "":
			spec = "name=" + tag.Get("json")

		default:
			spec = "name=" + field.Name
		}

		opts := strings.Split(spec, ",")
		for _, opt := range opts {
			kv := strings.SplitN(opt, "=", 2)
			switch {
			case len(kv) != 2:
				continue
			case kv[0] != "name":
				continue
			case kv[1] == "-", kv[1] == "":
				continue
			}

			from, to := val.Type().Field(i).Name, kv[1]
			if reverse {
				from, to = to, from
			}

			fields[from] = to
		}
	}

	return fields
}

// StructToMap copies `src` struct to the `dst` map.
// Behavior is similar to `StructToStruct`.
func StructToMap(
	filter FieldFilter,
	src interface{},
	dst map[string]interface{},
) error {
	srcVal := indirect(reflect.ValueOf(src))

	fields := getFieldMappingFromTags(srcVal, false)

	for i := 0; i < srcVal.NumField(); i++ {
		fieldName := srcVal.Type().Field(i).Name

		if _, ok := fields[fieldName]; !ok {
			continue
		}

		subFilter, ok := filter.Filter(fields[fieldName])
		if !ok {
			// Skip this field.
			continue
		}
		srcField, err := getField(src, fieldName)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to get the field %s from %T", fieldName, src))
		}

		fieldName = fields[fieldName]

		switch srcField.Kind() {
		case reflect.Ptr, reflect.Interface:
			if srcField.IsNil() {
				dst[fieldName] = nil
				continue
			}
			v := make(map[string]interface{})
			if err := StructToMap(subFilter, srcField.Interface(), v); err != nil {
				return err
			}
			dst[fieldName] = v

		case reflect.Array, reflect.Slice:
			// Check if it is an array of values (non-pointers).
			if srcField.Type().Elem().Kind() != reflect.Ptr {
				// Handle this array/slice as a regular non-nested data structure: copy it entirely to dst.
				if srcField.Len() > 0 {
					dst[fieldName] = srcField.Interface()
				} else {
					dst[fieldName] = []interface{}(nil)
				}
				continue
			}
			v := make([]map[string]interface{}, 0)
			// Iterate over items of the slice/array.
			for i := 0; i < srcField.Len(); i++ {
				subValue := srcField.Index(i)
				newDst := make(map[string]interface{})
				if err := StructToMap(subFilter, subValue.Interface(), newDst); err != nil {
					return err
				}
				v = append(v, newDst)
			}
			dst[fieldName] = v

		default:
			// Set a value on a map.
			dst[fieldName] = srcField.Interface()
		}
	}
	return nil
}

func getField(obj interface{}, name string) (reflect.Value, error) {
	objValue := reflectValue(obj)
	field := objValue.FieldByName(name)
	if !field.IsValid() {
		return reflect.ValueOf(nil), errors.Errorf("no such field: %s in obj %T", name, obj)
	}
	return field, nil
}

func reflectValue(obj interface{}) reflect.Value {
	var val reflect.Value

	if reflect.TypeOf(obj).Kind() == reflect.Ptr {
		val = reflect.ValueOf(obj).Elem()
	} else {
		val = reflect.ValueOf(obj)
	}

	return val
}

func indirect(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v
}
