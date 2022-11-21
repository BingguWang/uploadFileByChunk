package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"uploadFileByChunk/initRouter"
	"uploadFileByChunk/model"
)

// 其实是假的合并，以为我们没有保存分块再写入文件，而是直接写入，所以这里的合并其实相当于是确认
func MergeFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req MergeFileReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx := context.TODO()
		client := initRouter.GVA_REDIS

		if client.Exists(ctx, req.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount").Val() == 0 {
			w.Write([]byte("文件合并失败,尚未上传此文件"))
			return
		}
		hasUplaod, err := client.Get(ctx, req.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount").Int64()
		if err != nil {
			panic(err)
		}
		fmt.Println("已上传, ", hasUplaod)
		ret, _ := client.Get(ctx, fmt.Sprintf("%s", req.FileId)).Bytes()
		var info model.FileInfo
		json.Unmarshal(ret, &info)

		if int64(req.FileSize) != hasUplaod || len(info.FinishChunkIds) != req.ChunkCount {
			w.Write([]byte(fmt.Sprintf("文件合并失败,文件还为上传完毕：已上传 %f", float64(hasUplaod)/req.FileSize)))
			return
		}

		w.Write([]byte(fmt.Sprintf("文件上传成功")))
		return
	}
}

type MergeFileReq struct {
	FileId     string  `json:"fileId"`
	FileSize   float64 `json:"fileSize"`
	ChunkCount int     `json:"chunkCount"`
}
