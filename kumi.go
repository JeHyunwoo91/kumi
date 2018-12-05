package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/captv/kumi/model"
	"github.com/captv/kumi/modules/aws"
	"github.com/captv/kumi/modules/log"
	"github.com/captv/kumi/modules/redis"
)

const filepath_Prefix = "http://s3.ap-northeast-2.amazonaws.com/"
const acquisition_GET = "http://get-media-3.pooq.co.kr/v1/vods30/Acquisition/{{.ChannelID}}?contentId={{.ContentID}}&acquire={{.Acquire}}&period=0"
const acquisition_INSERT = "http://get-media.pooq.co.kr/v1/vods30/Acquisition/{{.ChannelID}}?contentId={{.ContentID}}&cornerId={{.CornerID}}&filepath={{.FilePath}}&acquire={{.Acquire}}&bitrate={{.Bitrate}}&isUse={{.IsUse}}&version={{.Version}}"
const acquisition_INSERT_V3 = "http://get-media-3.pooq.co.kr/v1/vods30/Acquisition/{{.ChannelID}}?contentId={{.ContentID}}&cornerId={{.CornerID}}&filepath={{.FilePath}}&acquire={{.Acquire}}&bitrate={{.Bitrate}}&isUse={{.IsUse}}&version={{.Version}}"

var logger = log.NewLogger("MAIN")

func checkError(err error) {
	if err != nil {
		logger.Error(err)
		os.Exit(1)
	}
}

func requestCMS(tpl interface{}, name string, _cmd string, method string) (rsp string, err error) {
	var cmd bytes.Buffer
	t := template.Must(template.New(name).Parse(_cmd))
	if err = t.Execute(&cmd, tpl); err != nil {
		return
	}

	var res *exec.Cmd
	if method == "GET" {
		res = exec.Command("curl", "-X", "GET", cmd.String())
	} else if method == "POST" {
		res = exec.Command("curl", "-X", "POST", "-d", `""`, cmd.String())
	} else {
		err = errors.New("undefined method")
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	res.Stdout = &out
	res.Stderr = &stderr
	err = res.Run()
	if err != nil {
		_err := errors.New(stderr.String())
		err = _err
		return
	}

	rsp = out.String()

	return
}

func targetBitrateInfo(list []model.MediaInfo, targetBitrate string) (res int, err error) {
	res = 0
	for _, mediainfo := range list {
		if mediainfo.Bitrate == targetBitrate {
			logger.Debug("Finding ", targetBitrate, " bitrate meta ", mediainfo.ContentID)
			res = 1
			break
		}
	}

	return
}

func checkCachedList(list []model.Meta) (nonExistList []model.Meta, err error) {
	logger.Debug("checkCachedList")
	for _, content := range list {

		val, _err := redis.Get(content.ContentID)
		if _err != nil && _err.Error() != "redigo: nil returned" {
			err = _err
			return
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
	setCachingList, err := checkCachedList(cachingList)
	if err != nil {
		return
	}

	tmp, _ := json.Marshal(setCachingList)
	logger.Debug("setCachingList for Set Redis: ", string(tmp))

	for _, obj := range setCachingList {
		err = redis.Set(obj.ContentID, []byte(obj.Version))
		if err != nil {
			return
		}
	}

	return
}

/*
	get-media-3를 호출하여, 5M, 15M meta 존재 여부를 확인 후,
	새로운 15M meta 삽입 되도록 list return
*/
func checkNewMeta(list []model.Meta) (newMetaList []model.Meta, err error) {
	logger.Debug("checkNewMeta")
	checkList := []model.Meta{}
Loop:
	for _, content := range list {
		tpl := &model.GetMediaParams{
			ChannelID: content.ChannelID,
			ContentID: content.ContentID,
			Acquire:   "Y",
		}

		// acquire=`Y`로 get-media 조회
		rsp, _err := requestCMS(tpl, "acquisition_GET", acquisition_GET, "GET")
		if _err != nil {
			err = _err
			return
		}

		data := model.Acquisition{}
		err = json.Unmarshal([]byte(rsp), &data)
		if err != nil {
			return
		}
		logger.Debug("Finished ", tpl.ContentID, " / ", tpl.Acquire, " acquisition_GET")

		if data.Message == "success" {
			logger.Debug("Success To get-media ", tpl.ContentID, " / ", tpl.Acquire)
		} else {
			logger.Error("Fail To get-media ", tpl.ContentID)
		}

		// 5M, 15M meta가 존재하는지 확인
		res5M, _ := targetBitrateInfo(data.Result.List, "5000")
		res15M, _ := targetBitrateInfo(data.Result.List, "15000")

		// 5M meta가 존재하지 않음. 입수 대상 아님
		if res5M == 0 {
			logger.Info(tpl.ContentID, " 5000 meta doesn't exist..., Skipped Ingest")
		} else if res5M == 1 && res15M == 0 {
			// 5M meta는 존재(acquire = Y), 15M meta가 존재하지 않을 때
			// D, N, P, F 중에 15M meta 가 존재하는지 확인
			logger.Info(tpl.ContentID, " 5M meta exist, but 15M meta doesn't exist...")
			chk := 0
			checkAcquireList := []string{"D", "N", "P", "F"}
			for idx, acquire := range checkAcquireList {
				_tpl := &model.GetMediaParams{
					ChannelID: tpl.ChannelID,
					ContentID: tpl.ContentID,
					Acquire:   acquire,
				}

				_rsp, _err := requestCMS(_tpl, "acquisition_GET", acquisition_GET, "GET")
				if _err != nil {
					err = _err
					return
				}

				_data := model.Acquisition{}
				err = json.Unmarshal([]byte(_rsp), &_data)
				if err != nil {
					return
				}
				logger.Debug(_tpl.ContentID, " / ", acquire, ": ", _data)
				cnt, _ := strconv.Atoi(_data.Result.Count)
				if cnt > 0 {
					_res15M, _ := targetBitrateInfo(_data.Result.List, "15000")
					if _res15M == 1 {
						chk = 1
						logger.Info("SKIP [", _tpl.ContentID, "] checkNewMeta because, 15000 bitrate meta has acquire ", acquire)
						// 15M meta가 존재하지만, redis에 cache 되었는지를 확인 하기 위해 checkList에 추가
						checkList = append(checkList, content)
					}
				}

				if (idx == len(checkAcquireList)-1) && chk == 0 {
					/*15M meta가 모든 acquire에 대해 존재하지 않으므로,
					입수 대상 list에 추가하여 return*/
					logger.Debug("[", _tpl.ContentID, "] Can't find 15M meta any acquire...")
					newMetaList = append(newMetaList, content)
				} else if chk == 1 {
					logger.Debug("[", _tpl.ContentID, "] find 15M meta at acquire ", acquire)
					continue Loop
				}
			}
			// 5M, 15M 모두 meta가 존재할 때
		} else {
			logger.Debug("SKIP [", tpl.ContentID, "] checkNewMeta because, 15M meta already exist acquire `Y`")
			// 이미 존재하는 15M meta지만, cache 되었는지 확인을 위해 checkList에 추가
			checkList = append(checkList, content)
		}
	}

	err = cachingHandler(checkList)
	if err != nil {
		return
	}

	return
}

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

		data := model.AquireReport{}

		rspv2, _err := requestCMS(tpl, "acquisition_INSERT", acquisition_INSERT, "POST")
		if _err != nil {
			err = _err
			return
		} else {
			if _err = json.Unmarshal([]byte(rspv2), &data); _err != nil {
				err = _err
				return
			} else if data.Result.Status == "FAIL" {
				logger.Debug(tpl.ContentID, " acquisition_INSERT result: ", data)
				err = errors.New("Fail to acquisition_INSERT " + tpl.ContentID)
				return
			}
		}

		rspv3, _err := requestCMS(tpl, "acquisition_INSERT_V3", acquisition_INSERT_V3, "POST")
		if _err != nil {
			err = _err
			return
		} else {
			if _err = json.Unmarshal([]byte(rspv3), &data); _err != nil {
				err = _err
				return
			} else if data.Result.Status == "FAIL" {
				err = errors.New("Fail to acquisition_INSERT_V3 " + tpl.ContentID)
				return
			}
		}
	}

	return
}

func main() {
	var channelList = []string{"K01", "K02"}
	for _, channel := range channelList {
		var bucket = "pooqcdn-cp-" + strings.ToLower(channel)
		var prefix = []string{"mp4",
			strings.ToUpper(channel) + "/",
		}

		// s3에서 각 bucket의 /mp4/{{.ChannelId}}/ 하위의 모든 directory list 정보를 가지고 온다
		contentList, err := aws.ListObjects(bucket, strings.Join(prefix, "/"))
		checkError(err)

		var MetaList []model.Meta
		for _, content := range contentList {
			var path = strings.Split(content, "/")
			var _meta model.Meta

			_meta.ChannelID = path[1]
			var _contentID = path[2][:4]
			if _contentID == path[1]+"_" {
				_meta.ContentID = path[2]
			} else {
				_meta.ContentID = path[1] + "_" + path[2]
			}
			_meta.CornerID = path[3]
			_meta.Version = "0"
			_meta.Bitrate = path[5]
			_meta.FilePath = filepath_Prefix + bucket + "/" + content
			_meta.IsUse = "Y"
			_meta.Acquire = "N"

			MetaList = append(MetaList, _meta)
		}

		/*
			redis에 cache 된 이력이 있는지 확인
			cache 되어 있지 않은 list를 추출
		*/
		nonExistList, err := checkCachedList(MetaList)
		checkError(err)

		/*
			nonExistList 중에서 get-media를 호출하여
			새로운 15M meta를 삽입하여야 하는지 판단
		*/
		newMetaList, err := checkNewMeta(nonExistList)
		checkError(err)

		// 새롭게 15M meta를 삽입할 list를 get-media를 통해 호출
		err = insertNewMeta(newMetaList)
		checkError(err)

		err = cachingHandler(newMetaList)
		checkError(err)

		currentTime := time.Now().Local()
		par, _ := json.Marshal(newMetaList)
		logger.Debug(currentTime.Format("2006-01-02"), "[", channel, "] ", "newMetaList: ", string(par))

		logger.Debug("End of KUMI ", channel)
	}
}
