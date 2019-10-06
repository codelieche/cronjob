package common

import "encoding/json"

// 反序列化Job
func UnpackByteToJob(value []byte) (job *Job, err error) {

	// 直接用json反序列化
	if err = json.Unmarshal(value, &job); err != nil {
		return
	} else {
		return job, nil
	}
}
