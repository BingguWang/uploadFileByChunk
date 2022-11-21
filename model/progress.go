package model

type Progress struct {
	FileSize      float64         `json:"fileSize"`
	FileHasUpload float64         `json:"fileHasUpload"`
	Chunks        []ChunkProgress `json:"chunks"`
}
type ChunkProgress struct {
	ChunkId   int     `json:"chunkId"`
	TotalSize float64 `json:"totalSize"` //切片总大小
	HasUpload float64 `json:"hasUpload"` //已传成功大小
	Rate      float64 `json:"rate"`      // 传输比例
}

type GetProgressReq struct {
	FileId string `json:"fileId"`
}
