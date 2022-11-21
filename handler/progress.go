package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"uploadFileByChunk/initRouter"
	"uploadFileByChunk/model"
	"uploadFileByChunk/utils"
)

func GetProgress() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := initRouter.GVA_REDIS
		var res model.Progress
		var req model.GetProgressReq
		ctx := context.TODO()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			panic(err.Error())
		}
		fmt.Println(utils.Tojson(req))
		// 获取Redis文件信息
		ret, _ := client.Get(ctx, fmt.Sprintf("%s", req.FileId)).Bytes()
		var info model.FileInfo
		json.Unmarshal(ret, &info)

		// 获取文件Redis进度信息
		count := client.Get(ctx, req.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount").Val()
		res.FileSize = info.FileSize
		c, _ := strconv.ParseFloat(count, 64)
		res.FileHasUpload = c

		// 获取切片Redis进度信息
		all := client.HGetAll(ctx, req.FileId+"_CHUNK_UPLOAD_PROGRESS").Val()
		for k, v := range all {
			// 获取切片的Redis信息
			rt, _ := client.Get(ctx, fmt.Sprintf("%s_%s", req.FileId, k)).Bytes()
			var cin model.ChunkInfo
			json.Unmarshal(rt, &cin)

			p := model.ChunkProgress{}
			split := strings.Split(k, "_")
			id, _ := strconv.Atoi(split[0])
			hu, _ := strconv.Atoi(v)
			p.ChunkId = id
			p.HasUpload = float64(hu)
			p.TotalSize = cin.CurrentChunkSize
			p.Rate = p.HasUpload / p.TotalSize
			res.Chunks = append(res.Chunks, p)
		}

		w.Write([]byte(utils.Tojson(res)))
	}
}
