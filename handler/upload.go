package handler

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"time"
	"uploadFileByChunk/initRouter"
	"uploadFileByChunk/model"
	"uploadFileByChunk/utils"
)

func SetupCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "*")
}
func UploadFileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		SetupCORS(&w)
		ctx := context.TODO()
		r.ParseMultipartForm(32 << 20)
		//获取上传文件
		file, handler, e := r.FormFile("file")
		if e != nil {
			fmt.Println(e)
			return
		}
		fmt.Println("成功获取到文件")
		fmt.Println(utils.Tojson(handler))
		defer file.Close()

		fileid := r.FormValue("identifier")
		fileName := r.FormValue("filename")
		destPath := r.FormValue("destPath")
		chunkID := r.FormValue("chunkNumber")
		chunkSize := r.FormValue("chunkSize")
		currentChunkSize := r.FormValue("currentChunkSize")
		chunkCount := r.FormValue("totalChunks")
		chunkMD5 := r.FormValue("chunkMD5")
		fileSize := r.FormValue("totalSize")
		chunk := model.Chunk{
			FileId:   fileid,
			Filename: fileName,
			DestPath: destPath,
			ChunkMD5: chunkMD5,
		}
		chunkID1, _ := strconv.Atoi(chunkID)
		count, _ := strconv.Atoi(chunkCount)
		chunk.ChunkNumber = chunkID1
		chunk.TotalChunks = count
		float, _ := strconv.ParseFloat(chunkSize, 64)
		size, _ := strconv.ParseFloat(fileSize, 64)
		currentSize, _ := strconv.ParseFloat(currentChunkSize, 64)
		chunk.ChunkSize = float
		chunk.TotalSize = size
		chunk.CurrentChunkSize = currentSize
		fmt.Println("chunk ----------- ", utils.Tojson(chunk))
		fmt.Println("参数解析完毕...")

		// 这里其实不需要计算的，这里计算只是为了输出看看，后期要删掉
		md := md5.New()
		io.Copy(md, file)
		md5Num := hex.EncodeToString(md.Sum(nil))
		fmt.Println("md5值为: ", md5Num)
		client := initRouter.GVA_REDIS

		InitChunkInfoInRedis(client, ctx, &chunk)
		InitChunkUploadProgressInfoInRedis(client, ctx, &chunk)
		InitChunkUploadStatusInfoInRedis(client, ctx, &chunk)
		InitFileInfoInRedis(client, ctx, &chunk)
		InitFileUploadByteInfoInRedis(client, ctx, &chunk)

		// 获取Redis里的file信息
		info, e1 := GetFileInfoInRedis(client, ctx, fmt.Sprintf("%s", chunk.FileId))
		if e1 != nil {
			panic(e1)
		}
		if info.IsDeprecated {
			DeprecateFileOp(&chunk, client, ctx)
		} else {
			fmt.Println("FinishChunkIds:", info.FinishChunkIds)
			if _, ok := info.FinishChunkIds[chunk.ChunkNumber]; ok {
				fmt.Println("已写入无需重传")
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		// 文件是否存在
		var f *os.File
		var err error
		filepath := chunk.DestPath + chunk.Filename
		fmt.Println("打开文件:", filepath)
		exists, err := utils.PathExists(filepath)
		if err != nil {
			panic(err)
		}
		if !exists {
			fmt.Println("文件不存在，开始创建文件...", filepath)
			// 创建文件
			f, err := os.Create(filepath)
			if err != nil {
				panic(err)
			}
			if err := f.Truncate(int64(chunk.TotalSize)); err != nil {
				panic(err)
			}
		}
		f, err = os.OpenFile(filepath, os.O_RDWR, 0777)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		i, err := client.HGet(ctx, chunk.FileId+"_CHUNK_UPLOAD_PROGRESS", fmt.Sprintf("%v_%s", chunk.ChunkNumber, chunk.ChunkMD5)).Int64()
		fmt.Println("已经传了: ", i)
		if i > int64(chunk.CurrentChunkSize) {
			fmt.Println("CurrentChunkSize ： ", chunk.CurrentChunkSize)
			panic("非法长度")
		}
		offset := (chunk.ChunkNumber-1)*(int(chunk.ChunkSize)) + int(i)
		fmt.Println("offset : ", offset)

		writeByteCount := WriteIntoFile(file, f, offset) // 返回写入的字节数
		//###################### 传入的内容写之后需要做如下操作

		cin, e := GetChunkInfoInRedis(client, ctx, fmt.Sprintf("%s_%v_%s", chunk.FileId, chunk.ChunkNumber, chunk.ChunkMD5))
		// 分片时否写完了
		if cin.EndIndex+writeByteCount == int(chunk.CurrentChunkSize) { // 此分块写完了
			md5Num, err := ValidateChunkMd5(filepath, &chunk)
			if err != nil {
				panic(err)
			}
			if md5Num != chunk.ChunkMD5 { // 分块不对，分块被丢弃
				fmt.Println("分块MD5校验失败, 分块被丢弃")
				// Redis内文件信息先更新
				if client.Exists(ctx, fmt.Sprintf("%s", chunk.FileId)).Val() == 1 {
					delete(info.FinishChunkIds, chunk.ChunkNumber) // 文件内记录先删除此分片
					client.Set(ctx, fmt.Sprintf("%s", chunk.FileId), utils.Tojson(info), 24*time.Hour)
				}
				// redis内文件进度信息更新
				if client.Exists(ctx, chunk.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount").Val() == 1 {
					if client.IncrBy(ctx, chunk.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount", -1*int64(cin.EndIndex)).Val() < 0 {
						client.HSet(ctx, chunk.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount", 0)
					}
				}
				// 删除此分片的Redis进度信息
				client.HDel(ctx, chunk.FileId+"_CHUNK_UPLOAD_PROGRESS", fmt.Sprintf("%v_%s", chunk.ChunkNumber, chunk.ChunkMD5))
				client.HDel(ctx, chunk.FileId+"_CHUNK_UPLOAD_STATUS", fmt.Sprintf("%v_%s", chunk.ChunkNumber, chunk.ChunkMD5))
				// 删除此分片的Redis信息
				client.Del(ctx, fmt.Sprintf("%s_%v_%s", chunk.FileId, chunk.ChunkNumber, chunk.ChunkMD5))
				w.Write([]byte(fmt.Sprintf("分块序号: %v,  前端计算的MD5: %s , 后端计算的MD5：%s", chunk.ChunkNumber, chunk.ChunkMD5, md5Num)))
				w.WriteHeader(http.StatusUnavailableForLegalReasons)
				return
			} else {
				// 更新文件情况
				fmt.Println("分片写完了，分片id为：", chunk.ChunkNumber)
				info.FinishChunkIds[chunk.ChunkNumber] = struct{}{}
				fmt.Println(utils.Tojson(info))
				client.Set(ctx, fmt.Sprintf("%s", chunk.FileId), utils.Tojson(info), 24*time.Hour)
			}
		}

		// 更新分片信息
		cin.EndIndex += writeByteCount
		if cin.EndIndex == int(chunk.CurrentChunkSize) {
			cin.ChunkUploadStatus = "finish"
		}
		client.Set(ctx, fmt.Sprintf("%s_%v_%s", chunk.FileId, chunk.ChunkNumber, chunk.ChunkMD5), utils.Tojson(cin), 24*time.Hour)
		// 写完分片，更新分片进度
		client.HIncrBy(ctx, chunk.FileId+"_CHUNK_UPLOAD_PROGRESS", fmt.Sprintf("%v_%s", chunk.ChunkNumber, chunk.ChunkMD5), int64(writeByteCount))
		client.HSet(ctx, chunk.FileId+"_CHUNK_UPLOAD_STATUS", fmt.Sprintf("%v_%s", chunk.ChunkNumber, chunk.ChunkMD5), "finish")

		// 更新文件进度
		if client.Exists(ctx, chunk.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount").Val() == 1 {
			client.IncrBy(ctx, chunk.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount", int64(writeByteCount))
		}
		return
	}
}

// 校验分块MD5
func ValidateChunkMd5(filepath string, chunk *model.Chunk) (string, error) {
	// 写完分片校验md5
	md5h := md5.New()
	// 需要从文件里读出分片部分的数据来计算md5
	f, err := os.OpenFile(filepath, os.O_RDWR, 0777)
	if err != nil {
		return "", err
	}
	buf := make([]byte, 1024)
	start := (chunk.ChunkNumber - 1) * (int(chunk.ChunkSize))
	var sum int64
	end := int(chunk.CurrentChunkSize) + start
	step := len(buf)
	for {
		//fmt.Println("sun: ", sum)
		//fmt.Println("start: ", start)
		if sum == int64(chunk.CurrentChunkSize) {
			fmt.Println("分片已全部读出到hash里: ", sum)
			break
		}
		if start+len(buf) > end {
			step = end - start
		}
		readN, _ := f.ReadAt(buf[:step], int64(start))
		//fmt.Println("从文件读取出字节数:", readN)
		//fmt.Println(string(buf[:readN]))
		reader := bytes.NewReader(buf[:readN])
		written, _ := io.Copy(md5h, reader)
		//fmt.Println("写入到hash里的字节数：", written)
		sum += written
		start += int(written)
	}
	md5Num := hex.EncodeToString(md5h.Sum(nil))
	fmt.Println("md5值为: ", md5Num)
	return md5Num, nil
}

// 把收到的内容写入到最终文件里
func WriteIntoFile(file multipart.File, f *os.File, offset int) int {
	// 缓存写入,发现个奇怪现象，file在读取的时候，读取有个指针的其实， 这个指针会移到末尾
	reader := bufio.NewReader(file)
	buf := make([]byte, 64)
	file.Seek(0, 0)        // 从文件流最开始读
	var writeByteCount int // 写入文件的字节数
	for {
		//fmt.Println("开始写入内容到文件....")
		// 读出
		n, err := reader.Read(buf) // 如果buf长度大于reader设置的缓存大小，就认为是大文件上传,会避免copy,直接从reader里read到buf,否则是copy到buf里
		//fmt.Println("读出:", n)
		if err == io.EOF {
			fmt.Println("上传完成")
			break
		}
		// 写入
		wn, e := f.WriteAt(buf[:n], int64(offset))
		if e != nil {
			panic(err)
		}
		//fmt.Println("写入字节数: ", wn)
		writeByteCount += wn
		offset += wn
	}
	fmt.Println("写入字节数--", writeByteCount)
	return writeByteCount
}

// 文件废弃时的操作
func DeprecateFileOp(chunk *model.Chunk, client *redis.Client, ctx context.Context) {
	fmt.Println("此文件已废弃，删除此文件以及分片Redis信息")
	fmt.Println("删除文件")
	os.Remove(chunk.DestPath + chunk.Filename)
	fmt.Println("删除分片Redis信息：进度信息，redis内分片信息")
	client.Del(ctx, chunk.FileId+"_CHUNK_UPLOAD_PROGRESS", fmt.Sprintf("%s_%s", chunk.FileId, chunk.ChunkMD5))
}

func InitFileUploadByteInfoInRedis(client *redis.Client, ctx context.Context, chunk *model.Chunk) {
	if client.Exists(ctx, chunk.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount").Val() == 0 {
		fmt.Println("不存在文件进度的key,初始化....")
		client.Set(ctx, chunk.FileId+"_FILE_UPLOAD_PROGRESS_ByteCount", 0, 0)
	}
}

func InitFileInfoInRedis(client *redis.Client, ctx context.Context, chunk *model.Chunk) {
	if client.Exists(ctx, fmt.Sprintf("%s", chunk.FileId)).Val() == 0 {
		fmt.Println("不存在文件的Redis信息,初始化....")
		// 文件Redis信息写入
		finfo := model.FileInfo{
			FileId:         chunk.FileId,
			FileName:       chunk.Filename,
			DestPath:       chunk.DestPath,
			ChunkCount:     chunk.TotalChunks,
			FileSize:       chunk.TotalSize,
			FinishChunkIds: make(map[int]struct{}),
		}
		client.Set(ctx, fmt.Sprintf("%s", chunk.FileId), utils.Tojson(finfo), 24*time.Hour)
	}
}

func InitChunkUploadStatusInfoInRedis(client *redis.Client, ctx context.Context, chunk *model.Chunk) {
	if !client.HExists(ctx, chunk.FileId+"_CHUNK_UPLOAD_STATUS", fmt.Sprintf("%v_%s", chunk.ChunkNumber, chunk.ChunkMD5)).Val() {
		fmt.Println("不存在分片上传状态的key,初始化....")
		client.HSet(ctx, chunk.FileId+"_CHUNK_UPLOAD_STATUS", fmt.Sprintf("%v_%s", chunk.ChunkNumber, chunk.ChunkMD5), "")
	}
	// 设置hash过期时间
	client.Expire(ctx, chunk.FileId+"_CHUNK_UPLOAD_PROGRESS", 24*time.Hour)
}

func InitChunkUploadProgressInfoInRedis(client *redis.Client, ctx context.Context, chunk *model.Chunk) {
	if !client.HExists(ctx, chunk.FileId+"_CHUNK_UPLOAD_PROGRESS", fmt.Sprintf("%v_%s", chunk.ChunkNumber, chunk.ChunkMD5)).Val() {
		fmt.Println("不存在分片上传进度的key,初始化....")
		client.HSet(ctx, chunk.FileId+"_CHUNK_UPLOAD_PROGRESS", fmt.Sprintf("%v_%s", chunk.ChunkNumber, chunk.ChunkMD5), 0)
	}
}

func InitChunkInfoInRedis(client *redis.Client, ctx context.Context, chunk *model.Chunk) {
	if client.Exists(ctx, fmt.Sprintf("%s_%v_%s", chunk.FileId, chunk.ChunkNumber, chunk.ChunkMD5)).Val() == 0 { // 不存在
		// 分片信息写入Redis
		info := model.ChunkInfo{
			FileId:            chunk.FileId,
			ChunkID:           chunk.ChunkNumber,
			ChunkSize:         chunk.ChunkSize,
			CurrentChunkSize:  chunk.CurrentChunkSize,
			ChunkMD5:          chunk.ChunkMD5,
			EndIndex:          0,
			ChunkUploadStatus: "writing",
			IsDeprecated:      false,
		}
		client.Set(ctx, fmt.Sprintf("%s_%v_%s", chunk.FileId, chunk.ChunkNumber, chunk.ChunkMD5), utils.Tojson(info), 24*time.Hour)
	}
}

/**
TODO 失效的文件何时删除, 删除文件的时机是啥，判定文件废弃的标准是啥？
*/

func GetFileInfoInRedis(client *redis.Client, ctx context.Context, key string) (*model.FileInfo, error) {
	ret, err := client.Get(ctx, fmt.Sprintf("%s", key)).Bytes()
	if err != nil {
		return nil, err
	}
	var info model.FileInfo
	json.Unmarshal(ret, &info)
	return &info, nil
}

func GetChunkInfoInRedis(client *redis.Client, ctx context.Context, key string) (*model.ChunkInfo, error) {
	ret, err := client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var info model.ChunkInfo
	json.Unmarshal(ret, &info)
	return &info, nil
}
