package swagger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

func mergeValue(path []string, patch map[string]interface{}, key string, value interface{}) interface{} {
	patchValue, patchHasValue := patch[key]

	if !patchHasValue {
		return value
	}

	_, patchValueIsObject := patchValue.(map[string]interface{})

	path = append(path, key)

	if _, ok := value.(map[string]interface{}); ok {
		if !patchValueIsObject {
			return value
		}

		return mergeObjects(value, patchValue, path)
	}

	if _, ok := value.([]interface{}); ok && patchValueIsObject {
		return mergeObjects(value, patchValue, path)
	}

	return patchValue
}

func mergeObjects(data, patch interface{}, path []string) interface{} {
	if patchObject, ok := patch.(map[string]interface{}); ok {
		if dataArray, ok := data.([]interface{}); ok {
			ret := make([]interface{}, len(dataArray))

			for i, val := range dataArray {
				ret[i] = mergeValue(path, patchObject, strconv.Itoa(i), val)
			}

			return ret
		} else if dataObject, ok := data.(map[string]interface{}); ok {
			ret := make(map[string]interface{})

			for k, v := range dataObject {
				ret[k] = mergeValue(path, patchObject, k, v)
			}
			for k, v := range patchObject {

				if _, patchOnly := ret[k]; !patchOnly {
					ret[k] = v
				}
			}

			return ret
		}
	}

	return data
}

// MergeBytes merges patch document buffer to data document buffer
//
// Returning merged document buffer and error if any
func MergeBytes(dataBuff, patchBuff []byte) (mergedBuff []byte, err error) {
	var data, patch, merged interface{}

	err = unmarshalJSON(dataBuff, &data)
	if err != nil {
		err = fmt.Errorf("error in data JSON: %v", err)
		return
	}

	err = unmarshalJSON(patchBuff, &patch)
	if err != nil {
		err = fmt.Errorf("error in patch JSON: %v", err)
		return
	}

	merged = mergeObjects(data, patch, nil)

	mergedBuff, err = json.Marshal(merged)
	if err != nil {
		err = fmt.Errorf("error writing merged JSON: %v", err)
	}

	return
}

func unmarshalJSON(buff []byte, data interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(buff))
	decoder.UseNumber()

	return decoder.Decode(data)
}
