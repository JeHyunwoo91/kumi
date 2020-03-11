package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/captv/kumi_azure/model"
	"github.com/captv/kumi_azure/modules/aws"
	"github.com/captv/kumi_azure/modules/log"
	"github.com/captv/kumi_azure/modules/redis"
	"github.com/joho/godotenv"
)

const awsS3DomainPrefix = "http://s3.ap-northeast-2.amazonaws.com/"
const azureBlobDomainPrefix = "https://.blob.core.windows.net/vod"

const acquisitionGET = "http://get-media-vod.internal.wavve.com/v1/vods30/Acquisition/{{.ChannelID}}?contentId={{.ContentID}}&acquire={{.Acquire}}&period=0"
const acquisitionINSERT = "http://get-media-vod.internal.wavve.com/v1/vods30/Acquisition/{{.ChannelID}}?contentId={{.ContentID}}&cornerId={{.CornerID}}&filepath={{.FilePath}}&acquire={{.Acquire}}&bitrate={{.Bitrate}}&isUse={{.IsUse}}&version={{.Version}}"

var logger = log.NewLogger("MAIN")

func checkError(err error) {
	if err != nil {
		logger.Error(err)
		os.Exit(1)
	}
}

func requestCMS(tpl interface{}, name string, _cmd string, method string) (rsp string, err error) {
	var url bytes.Buffer
	t := template.Must(template.New(name).Parse(_cmd))
	if err = t.Execute(&url, tpl); err != nil {
		return
	}

	req, err := http.NewRequest(
		method,
		url.String(),
		nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return
	}

	if resp.StatusCode > 400 {

	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	rsp = string(b)
	logger.Debug("rsp: ", rsp)
	return
}

func targetBitrateInfo(list []model.MediaInfo, targetBitrate string) (res bool) {
	for _, mediainfo := range list {
		if mediainfo.Bitrate == targetBitrate {
			logger.Debug("Finding ", targetBitrate, " bitrate meta ", mediainfo.ContentID)
			res = true
			break
		}
	}

	return
}

func checkCachedList(list []model.Meta) (nonExistList []model.Meta, err error) {
	logger.Debug("check redis Cached List")
	for _, content := range list {
		val, _err := redis.Get(content.ContentID)

		fmt.Println("Get Err: ", _err)
		if _err != nil && _err.Error() != "redigo: nil returned" {
			return []model.Meta{}, _err
		}

		logger.Debug(content.ContentID+" check cached val: ", string(val))
		if val == nil || len(val) == 0 {
			nonExistList = append(nonExistList, content)
		}
	}

	return
}

func cachingHandler(cachingList []model.Meta) (err error) {
	logger.Debug("cachingHandler")

	uncachedList, err := checkCachedList(cachingList)
	if err != nil {
		return
	}

	_uncachedList, _ := json.Marshal(uncachedList)
	logger.Debug("uncachedList for Set Redis[", len(uncachedList), "]: ", string(_uncachedList))

	for _, meta := range uncachedList {
		err = redis.Set(meta.ContentID, []byte(meta.Version))
		if err != nil {
			return
		}
	}

	return
}

/*
	get-media GET API를 호출하여, 5M, 15M 각 meta 존재 여부를 파악.
	5M meta만 존재하여 15M meta 추가가 필요한 meta를 선출.

	list:			redis에 key로 저장되지 않은, content meta list
	newMetaList: 	15M meta를 get-media insert API를 통해 추가 할 meta list
*/
func checkMeta(list []model.Meta) (newMetaList []model.Meta, err error) {
	logger.Debug("check New Meta")
	var checkList []model.Meta

	for _, content := range list {
		tpl := &model.GetMediaParams{
			ChannelID: content.ChannelID,
			ContentID: content.ContentID,
			Acquire:   "A", // Querying All Acquire Status
		}

		// get-media 조회
		rsp, err := requestCMS(tpl, "acquisitionGET", acquisitionGET, "GET")
		if err != nil {
			return []model.Meta{}, err
		}

		var data model.Acquisition
		err = json.Unmarshal([]byte(rsp), &data)
		if err != nil {
			return []model.Meta{}, err
		}

		logger.Debug("Finished ", tpl.ContentID, " / ", tpl.Acquire, " acquisitionGET")

		if data.Message == "success" {
			logger.Debug("Success To get-media ", tpl.ContentID, " / ", tpl.Acquire)
		} else {
			logger.Error("Fail To get-media ", tpl.ContentID)
		}

		// 5M, 15M meta가 존재하는지 확인
		res5M := targetBitrateInfo(data.Result.List, "5000")
		res15M := targetBitrateInfo(data.Result.List, "15000")

		// 5M meta가 존재하지 않음. 입수 대상 아님
		if !res5M {
			logger.Info(tpl.ContentID, " 5M meta doesn't exist..., Skipped inserting meta")
		} else if res5M && !res15M {
			// acquire `A`일 때 5M meta는 존재하지만, 15M meta가 존재하지 않을 때
			logger.Info(tpl.ContentID, " 5M meta exist, but 15M meta doesn't exist...")
			newMetaList = append(newMetaList, content)
		} else { // 5M, 15M 모두 meta가 존재할 때
			logger.Debug("Skip [", tpl.ContentID, "] checkMeta because, 15M meta already exist in acquire `Y`")
			// 이미 존재하는 15M meta지만, cache 되었는지 확인을 위해 checkList에 추가
			checkList = append(checkList, content)
		}
	}

	err = cachingHandler(checkList)
	if err != nil {
		return
	}

	// return newMetaList
	return
}

/*
	POST Method를 통해 15M meta를 insert
*/
func insertNewMeta(insertList []model.Meta) (err error) {
	logger.Debug("insertNewMeta")

	for _, meta := range insertList {
		tpl := &model.Meta{
			ChannelID: meta.ChannelID,
			ContentID: meta.ContentID,
			CornerID:  meta.CornerID,
			FilePath:  meta.FilePath,
			Acquire:   meta.Acquire,
			Bitrate:   meta.Bitrate,
			IsUse:     meta.IsUse,
			Version:   meta.Version,
		}

		data := model.AcquireReport{}

		rsp, _err := requestCMS(tpl, "acquisition_INSERT", acquisitionINSERT, "POST")
		if _err != nil {
			err = _err
			return
		} else {
			if _err = json.Unmarshal([]byte(rsp), &data); _err != nil {
				err = _err
				return
			} else if data.Result.Status == "FAIL" {
				err = errors.New("Fail to acquisition_INSERT" + tpl.ContentID)
				return
			}
		}
	}

	return
}

func init() {
	if err := godotenv.Load(); err != nil {
		logger.Error("Error Load environment configuration:", err.Error())
	}
}

func main() {
	logger.Info("Go kumi_Azure!")
	channelList := strings.Split(os.Getenv("CHANNEL_LIST"), ",")
	for _, channel := range channelList {
		var contentList []string
		var err error
		var fileURLPrefix string

		if os.Getenv("REF_STORAGE_TYPE") == "S3" {
			logger.Debug("Referencing S3 Bucket")
			bucket := "pooqcdn-cp-" + strings.ToLower(channel)
			prefix := path.Join("mp4", strings.ToUpper(channel)+"/")
			fileURLPrefix = awsS3DomainPrefix + bucket

			contentList, err = aws.ListObjects(bucket, prefix)
		} else if os.Getenv("REF_STORAGE_TYPE") == "Blob" {
			logger.Debug("Referencing Azure Blob")

			fileURLPrefix = azureBlobDomainPrefix
		} else {
			err = errors.New("Error unsupported storage type")
		}

		checkError(err)
		logger.Debug("contentList[", len(contentList), "]:", contentList)

		var metaList []model.Meta
		for _, content := range contentList {
			var meta = strings.Split(content, "/")
			var _meta model.Meta

			_meta.ChannelID = meta[1]
			var _contentID = meta[2][:4]
			if _contentID == meta[1]+"_" {
				_meta.ContentID = meta[2]
			} else {
				_meta.ContentID = meta[1] + "_" + meta[2]
			}
			_meta.CornerID = meta[3]
			_meta.Version = "0"
			_meta.Bitrate = meta[5]
			_meta.FilePath = path.Join(fileURLPrefix, content)
			_meta.IsUse = "Y"
			_meta.Acquire = "N"

			metaList = append(metaList, _meta)
		}

		/*
			redis에 cache 된 이력이 있는지 확인
			cache 되어 있지 않은 list를 추출
		*/
		nonExistList, err := checkCachedList(metaList)
		checkError(err)
		logger.Debug("nonExistList[", len(nonExistList), "]: ", nonExistList)

		/*
			nonExistList 중에서 get-media를 호출하여
			새로운 15M meta를 삽입하여야 하는지 판단
		*/
		newMetaList, err := checkMeta(nonExistList)
		checkError(err)
		logger.Debug("newMetaList[", len(newMetaList), "]:", newMetaList)

		// 15M meta를 입력 할 meta list를 get-media / insert API를 통해 호출
		err = insertNewMeta(newMetaList)
		checkError(err)

		// 15M meta 추가 후, Redis caching 진행
		err = cachingHandler(newMetaList)
		checkError(err)

		currentTime := time.Now().Local()
		par, _ := json.Marshal(newMetaList)
		logger.Debug(currentTime.Format("2006-01-02"), " [", channel, "] to be added 15M meta[", len(newMetaList), "]:", string(par))

		logger.Debug("End of KUMI_Azure ", channel)
	}
}
