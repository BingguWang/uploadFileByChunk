package model

// 用于接收分片
type Chunk struct {
	ChunkNumber      int     `json:"chunkNumber"`      // 当前分块的序号， 从1开始
	TotalChunks      int     `json:"totalChunks"`      // 分块总数
	ChunkSize        float64 `json:"chunkSize"`        // 分片的大小
	CurrentChunkSize float64 `json:"currentChunkSize"` // 当前块的实际大小
	TotalSize        float64 `json:"totalSize"`        // 文件的总大小
	FileId           string  `json:"identifier"`       // 文件名
	Filename         string  `json:"filename"`         // 文件名
	RelativePath     string  `json:"relativePath"`     // 上传文件夹的时候,文件的相对路径属性
	DestPath         string  `json:"destPath"`         // 文件要保存的路径
	ChunkMD5         string  `json:"chunkMD5"`         // 分片MD5
}

// 存入Redis里的结构,key应该是md5
type ChunkInfo struct {
	FileId            string  `json:"fileId"`            // 文件UUID
	ChunkID           int     `json:"chunkID"`           // 分片的编号，从0开始
	ChunkSize         float64 `json:"chunkSize"`         // 分片的大小，字节数
	ChunkMD5          string  `json:"chunkMD5"`          // 分片MD5
	EndIndex          int     `json:"endIndex"`          // 分片已写入文件的字节数
	ChunkUploadStatus string  `json:"chunkUploadStatus"` // 分片上传状态, writing写入到文件中 ,finish已完整写入到文件内
	IsDeprecated      bool    `json:"isDeprecated"`      // 分片是否作废，作废就需要重传这个分片，即使已被写入到文件中了
	CurrentChunkSize  float64 `json:"currentChunkSize"`  // 当前块的实际大小

}

// 写入Redis的文件的信息
type FileInfo struct {
	FileId         string           `json:"fileId"`         // 操作文件ID，随机生成的UUID
	FileName       string           `json:"fileName"`       // 文件名
	DestPath       string           `json:"destPath"`       // 文件要保存的路径
	ChunkCount     int              `json:"chunkCount"`     // 分片数
	FileSize       float64          `json:"fileSize"`       // 文件大小
	FinishChunkIds map[int]struct{} `json:"finishChunkIds"` // 已写入到文件的分片id
	FileStatus     string           `json:"fileStatus"`     // 文件上传状态, writing未完成 ,finish已完整存入
	IsDeprecated   bool             `json:"isDeprecated"`   // 文件是否作废
}
