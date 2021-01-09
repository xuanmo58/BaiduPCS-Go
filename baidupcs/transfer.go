package baidupcs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

func (pcs *BaiduPCS) GenerateShareQueryURL(subPath string, params map[string]string) *url.URL {
	shareURL := &url.URL{
		Scheme: GetHTTPScheme(true),
		Host:   PanBaiduCom,
		Path:   "/share/" + subPath,
	}
	uv := shareURL.Query()
	uv.Set("app_id", PanAppID)
	uv.Set("channel", "chunlei")
	uv.Set("clienttype", "0")
	uv.Set("web", "1")
	for key, value := range params {
		uv.Set(key, value)
	}

	shareURL.RawQuery = uv.Encode()
	return shareURL
}

func (pcs *BaiduPCS) ExtractShareInfo(metajsonstr string) (res map[string]string) {
	res = make(map[string]string)
	if !strings.Contains(metajsonstr, "server_filename") {
		res["ErrMsg"] = "获取分享文件详情失败"
		return
	}
	errno := gjson.Get(metajsonstr, `file_list.errno`).Int()
	if errno != 0 {
		res["ErrMsg"] = "提取码错误"
		return
	}
	res["filename"] = gjson.Get(metajsonstr, `file_list.list.0.server_filename`).String()
	fsid_list := gjson.Get(metajsonstr, `file_list.list.#.fs_id`).Array()
	var fids_str string = "["
	if len(fsid_list) > 1 {
		res["filename"] += " 等多个文件"
	}
	for _, sid := range fsid_list {
		fids_str += sid.String() + ","
	}

	res["shareid"] = gjson.Get(metajsonstr, `shareid`).String()
	res["from"] = gjson.Get(metajsonstr, `uk`).String()
	res["bdstoken"] = gjson.Get(metajsonstr, `bdstoken`).String()
	shareUrl := &url.URL{
		Scheme: GetHTTPScheme(true),
		Host:   PanBaiduCom,
		Path:   "/share/transfer",
	}
	uv := shareUrl.Query()
	uv.Set("app_id", PanAppID)
	uv.Set("channel", "chunlei")
	uv.Set("clienttype", "0")
	uv.Set("web", "1")
	for key, value := range res {
		uv.Set(key, value)
	}
	res["ErrMsg"] = "0"
	res["fs_id"] = fids_str[:len(fids_str)-1] + "]"
	shareUrl.RawQuery = uv.Encode()
	res["shareUrl"] = shareUrl.String()
	return
}

func (pcs *BaiduPCS) PostShareQuery(url string, surl string, data map[string]string) (res map[string]string) {
	dataReadCloser, panError := pcs.sendReqReturnReadCloser(reqTypePan, OperationShareFileSavetoLocal, http.MethodPost, url, data, map[string]string{
		"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/76.0.3809.100 Safari/537.36",
		"Content-Type": "application/x-www-form-urlencoded",
		"Referer":      fmt.Sprintf("https://pan.baidu.com/share/init?surl=%s", surl),
	})
	res = make(map[string]string)
	if panError != nil {
		res["ErrMsg"] = "提交分享项查询请求时发生错误"
		return
	}
	defer dataReadCloser.Close()
	body, _ := ioutil.ReadAll(dataReadCloser)
	errno := gjson.Get(string(body), `errno`).String()
	if errno != "0" {
		res["ErrMsg"] = "分享码错误"
		return
	}
	res["ErrMsg"] = "0"
	return
}

func (pcs *BaiduPCS) AccessSharePage(featurestr string, first bool) (tokens map[string]string) {
	tokens = make(map[string]string)
	tokens["ErrMsg"] = "0"
	headers := make(map[string]string)
	headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/76.0.3809.100 Safari/537.36"
	headers["Referer"] = "https://pan.baidu.com/disk/home"
	if !first {
		headers["Referer"] = fmt.Sprintf("https://pan.baidu.com/share/init?surl=%s", featurestr[1:])
	}
	sharelink := fmt.Sprintf("https://pan.baidu.com/s/%s", featurestr)

	dataReadCloser, panError := pcs.sendReqReturnReadCloser(reqTypePan, OperationShareFileSavetoLocal, http.MethodGet, sharelink, nil, headers)

	if panError != nil {
		tokens["ErrMsg"] = "访问分享页失败"
		return
	}
	defer dataReadCloser.Close()
	body, _ := ioutil.ReadAll(dataReadCloser)
	not_found_flag := strings.Contains(string(body), "platform-non-found")
	error_page_title := strings.Contains(string(body), "error-404")
	if error_page_title {
		tokens["ErrMsg"] = "页面不存在"
		return
	}
	if not_found_flag {
		tokens["ErrMsg"] = "分享链接已失效"
		return
	} else {
		re, _ := regexp.Compile(`yunData\.setData\((\{"loginstate.+?\})\);`)
		sub := re.FindSubmatch(body)
		if len(sub) < 2 {
			tokens["ErrMsg"] = "分享页面解析失败"
			return
		}
		tokens["metajson"] = string(sub[1])
		return
	}

}

func (pcs *BaiduPCS) GenerateRequestQuery(mode string, params map[string]string) (res map[string]string) {
	res = make(map[string]string)
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/76.0.3809.100 Safari/537.36",
		"Referer":    params["referer"],
	}
	if mode == "POST" {
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	}
	postdata := make(map[string]string)
	postdata["fsidlist"] = params["fs_id"]
	postdata["path"] = params["path"]
	dataReadCloser, panError := pcs.sendReqReturnReadCloser(reqTypePan, OperationShareFileSavetoLocal, mode, params["shareUrl"], postdata, headers)
	if panError != nil {
		res["ErrMsg"] = "网络错误"
		return
	}
	defer dataReadCloser.Close()
	body, err := ioutil.ReadAll(dataReadCloser)
	res["ErrMsg"] = "0"
	if err != nil {
		res["ErrMsg"] = "未知错误"
		return
	}
	if !gjson.Valid(string(body)) {
		res["ErrMsg"] = "返回json解析错误"
		return
	}
	errno := gjson.Get(string(body), `errno`).Int()
	if errno != 0 {
		res["ErrMsg"] = "获取分享项元数据错误"
		if mode == "POST" && errno == 12 {
			path := gjson.Get(string(body), `info.0.path`).String()
			_, file := filepath.Split(path)
			_errno := gjson.Get(string(body), `info.0.errno`).Int()
			if _errno == -33 {
				filenum := gjson.Get(string(body), `target_file_nums`).Int()
				userlimit := gjson.Get(string(body), `target_file_nums_limit`).Int()
				res["ErrMsg"] = fmt.Sprintf("转存文件数%d超过当前用户上限, 当前用户单次最大转存数%d", filenum, userlimit)
			} else if _errno == -30 {
				res["ErrMsg"] = fmt.Sprintf("当前目录下已有%s同名文件/文件夹", file)
			} else {
				res["ErrMsg"] = fmt.Sprintf("未知错误, 错误代码%d", _errno)
			}
		} else {
			res["ErrMsg"] = fmt.Sprintf("未知错误, 错误代码%d", errno)
		}
		return
	}
	_, res["filename"] = filepath.Split(gjson.Get(string(body), `info.0.path`).String())
	if len(gjson.Get(string(body), `info.#.fsid`).Array()) > 1 {
		res["filename"] += "等多个文件"
	}
	return
}
