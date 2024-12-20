package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2"
	"github.com/gin-gonic/gin"
)

var mobReFull, mobRe4 *regexp.Regexp
var clientError, serverError, csrfError, redirectedErr error
var requestBrowserID BrowserIDBuilder

func init() {
	// https://stackoverflow.com/a/11381730
	mobReFull = regexp.MustCompile(`(?i)(android|bb\d+|meego).+mobile|avantgo|bada\/|blackberry|blazer|compal|elaine|fennec|hiptop|iemobile|ip(hone|od)|iris|kindle|lge |maemo|midp|mmp|mobile.+firefox|netfront|opera m(ob|in)i|palm( os)?|phone|p(ixi|re)\/|plucker|pocket|psp|series(4|6)0|symbian|treo|up\.(browser|link)|vodafone|wap|windows ce|xda|xiino`)
	mobRe4 = regexp.MustCompile(`(?i)1207|6310|6590|3gso|4thp|50[1-6]i|770s|802s|a wa|abac|ac(er|oo|s\-)|ai(ko|rn)|al(av|ca|co)|amoi|an(ex|ny|yw)|aptu|ar(ch|go)|as(te|us)|attw|au(di|\-m|r |s )|avan|be(ck|ll|nq)|bi(lb|rd)|bl(ac|az)|br(e|v)w|bumb|bw\-(n|u)|c55\/|capi|ccwa|cdm\-|cell|chtm|cldc|cmd\-|co(mp|nd)|craw|da(it|ll|ng)|dbte|dc\-s|devi|dica|dmob|do(c|p)o|ds(12|\-d)|el(49|ai)|em(l2|ul)|er(ic|k0)|esl8|ez([4-7]0|os|wa|ze)|fetc|fly(\-|_)|g1 u|g560|gene|gf\-5|g\-mo|go(\.w|od)|gr(ad|un)|haie|hcit|hd\-(m|p|t)|hei\-|hi(pt|ta)|hp( i|ip)|hs\-c|ht(c(\-| |_|a|g|p|s|t)|tp)|hu(aw|tc)|i\-(20|go|ma)|i230|iac( |\-|\/)|ibro|idea|ig01|ikom|im1k|inno|ipaq|iris|ja(t|v)a|jbro|jemu|jigs|kddi|keji|kgt( |\/)|klon|kpt |kwc\-|kyo(c|k)|le(no|xi)|lg( g|\/(k|l|u)|50|54|\-[a-w])|libw|lynx|m1\-w|m3ga|m50\/|ma(te|ui|xo)|mc(01|21|ca)|m\-cr|me(rc|ri)|mi(o8|oa|ts)|mmef|mo(01|02|bi|de|do|t(\-| |o|v)|zz)|mt(50|p1|v )|mwbp|mywa|n10[0-2]|n20[2-3]|n30(0|2)|n50(0|2|5)|n7(0(0|1)|10)|ne((c|m)\-|on|tf|wf|wg|wt)|nok(6|i)|nzph|o2im|op(ti|wv)|oran|owg1|p800|pan(a|d|t)|pdxg|pg(13|\-([1-8]|c))|phil|pire|pl(ay|uc)|pn\-2|po(ck|rt|se)|prox|psio|pt\-g|qa\-a|qc(07|12|21|32|60|\-[2-7]|i\-)|qtek|r380|r600|raks|rim9|ro(ve|zo)|s55\/|sa(ge|ma|mm|ms|ny|va)|sc(01|h\-|oo|p\-)|sdk\/|se(c(\-|0|1)|47|mc|nd|ri)|sgh\-|shar|sie(\-|m)|sk\-0|sl(45|id)|sm(al|ar|b3|it|t5)|so(ft|ny)|sp(01|h\-|v\-|v )|sy(01|mb)|t2(18|50)|t6(00|10|18)|ta(gt|lk)|tcl\-|tdg\-|tel(i|m)|tim\-|t\-mo|to(pl|sh)|ts(70|m\-|m3|m5)|tx\-9|up(\.b|g1|si)|utst|v400|v750|veri|vi(rg|te)|vk(40|5[0-3]|\-v)|vm40|voda|vulc|vx(52|53|60|61|70|80|81|83|85|98)|w3c(\-| )|webc|whit|wi(g |nc|nw)|wmlb|wonu|x700|yas\-|your|zeto|zte\-`)
	clientError = errors.New("client error")
	serverError = errors.New("server error")
	csrfError = errors.New("csrf error")
	redirectedErr = errors.New("redirected")

	requestBrowserID = NewDefaultBrowserIDBuilder()
}

type APIRequest struct {
	mdw  *Mindwell
	ctx  *gin.Context
	err  error
	resp *http.Response
	read bool // whether resp is read
	data map[string]interface{}
	aTok string
	uid2 string
	st   *ServerTiming
}

func NewRequest(mdw *Mindwell, ctx *gin.Context) *APIRequest {
	st := NewServerTiming()
	st.Add("api").Start()

	api := &APIRequest{
		mdw:  mdw,
		ctx:  ctx,
		read: false,
		st:   st,
	}

	api.RefreshAuth()

	return api
}

func (api *APIRequest) Server() *Mindwell {
	return api.mdw
}

func (api *APIRequest) Error() error {
	return api.err
}

func (api *APIRequest) SkipError() {
	if api.err != redirectedErr {
		api.err = nil
	}
}

func (api *APIRequest) StatusCode() int {
	if api.resp == nil {
		return 200
	}

	return api.resp.StatusCode
}

func (api *APIRequest) Data() map[string]interface{} {
	if api.err != nil {
		return nil
	}

	if api.data == nil {
		api.data = api.parseResponse()
	}

	return api.data
}

func (api *APIRequest) ClearData() {
	if api.err != nil {
		return
	}

	api.data = nil
}

func (api *APIRequest) SetData(key string, value interface{}) {
	if api.err != nil {
		return
	}

	data := api.Data()
	if data == nil {
		data = make(map[string]interface{})
		api.data = data
	}

	data[key] = value
}

func (api *APIRequest) SetDataFromQuery(key, defaultValue string) {
	if api.err != nil {
		return
	}

	value, ok := api.ctx.GetQuery(key)
	if !ok {
		value = defaultValue
	}

	api.SetData("__"+key, value)
}

func (api *APIRequest) FormString(key string) string {
	if api.err != nil {
		return ""
	}

	return api.ctx.PostForm(key)
}

func (api *APIRequest) Header(key string) string {
	return api.ctx.GetHeader(key)
}

func (api *APIRequest) SetRequestData(args url.Values) {
	data := args.Encode()
	api.ctx.Request.Body = ioutil.NopCloser(strings.NewReader(data))
	api.ctx.Request.ContentLength = int64(len(data))
	api.ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
}

func (api *APIRequest) SetScrollHrefs() {
	api.SetScrollHrefsWithData(api.ctx.Request.URL.Path, api.Data())
}

func (api *APIRequest) SetScrollHrefsWithData(webPath string, data map[string]interface{}) {
	_, setBefore := api.ctx.Params.Get("before")
	_, setAfter := api.ctx.Params.Get("after")

	query := api.ctx.Request.URL.Query()
	query.Del("after")
	query.Del("before")

	if setBefore || setAfter == setBefore {
		if has, ok := data["hasBefore"].(bool); has && ok {
			if before, ok := data["nextBefore"].(string); ok {
				query.Set("before", before)
				href := webPath + "?" + query.Encode()
				api.SetData("beforeHref", href)
				query.Del("before")
			}
		}
	}

	if setAfter || setAfter == setBefore {
		if after, ok := data["nextAfter"].(string); ok {
			query.Set("after", after)
		}

		href := webPath + "?" + query.Encode()
		api.SetData("afterHref", href)
		query.Del("after")
	}
}

func (api *APIRequest) SetCsrfToken(action string) {
	client := api.ClientIP()
	token := api.mdw.CreateCsrfToken(action, client)
	path := strings.Split(action, "/")
	name := path[len(path)-1]
	api.SetData("__csrf_"+name, token)
}

func (api *APIRequest) ReadBody() []byte {
	req := api.ctx.Request
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		api.mdw.LogWeb().Error(err.Error())
	}

	api.SetBody(body)

	return body
}

func (api *APIRequest) SetBody(body []byte) {
	api.ctx.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
}

func (api *APIRequest) CheckCsrfTokenRead() {
	req := api.ctx.Request
	token := req.PostFormValue("csrf")
	action := req.URL.Path
	client := api.ClientIP()

	if err := api.mdw.CheckCsrfToken(token, action, client); err != nil {
		api.mdw.LogWeb().Error(err.Error())

		api.SetData("code", 419)
		api.SetData("message", "Время сессии истекло. Необходимо перезагрузить страницу.")
		api.err = csrfError
	}
}

func (api *APIRequest) CheckCsrfToken() {
	body := api.ReadBody()
	api.CheckCsrfTokenRead()

	if api.err != csrfError {
		api.SetBody(body)
	}
}

func (api *APIRequest) HasUserKey() bool {
	return api.aTok != ""
}

func (api *APIRequest) setUserKey(allowNoKey bool) bool {
	if api.err != nil {
		return false
	}

	if api.HasUserKey() {
		return true
	}

	token, err := api.Cookie("at")
	if err != nil {
		return allowNoKey
	}

	api.aTok = token.Value

	uid2, err := api.ctx.Cookie("uid2")
	if err != nil {
		uid2 = api.mdw.Uid2(api.aTok)
	}
	api.uid2 = uid2

	return true
}

func (api *APIRequest) authToken() string {
	if api.aTok != "" {
		return api.aTok
	}

	return api.mdw.AppToken()
}

func (api *APIRequest) RefreshAuth() bool {
	if api.err != nil {
		return false
	}

	if api.IsAjax() || !api.IsGet() || !api.IsWebRequest() {
		return false
	}

	if !api.IsAuthRefreshRequired() {
		return false
	}

	if !api.IsAuthRefreshPossible() {
		return false
	}

	api.RedirectToAuth("/refresh?to=" + api.NextRedirect())
	api.err = redirectedErr

	return true
}

func (api *APIRequest) RequestRefreshAuth() {
	if api.IsAuthRefreshPossible() && api.IsGet() && api.IsWebRequest() {
		api.RedirectToAuth("/refresh?to=" + api.NextRedirect())
	} else {
		api.Redirect("/index.html?to=" + api.NextRedirect())
	}
}

func (api *APIRequest) doNamed(req *http.Request, name string) {
	defer api.st.Add(name).Start().Stop()

	api.mdw.LogWeb().Debug("api",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
	)

	api.resp, api.err = http.DefaultTransport.RoundTrip(req)
	if api.err != nil {
		api.mdw.LogWeb().Error(api.err.Error())
		api.err = nil
		api.SetData("code", 500)
		api.SetData("message", "Произошла внутренняя ошибка")
		api.err = serverError
	}

	api.read = false
}

func (api *APIRequest) do(req *http.Request) {
	api.doNamed(req, "main")
}

func (api *APIRequest) QueryCookie() {
	link := api.ctx.Request.URL
	path := strings.Split(link.Path, "/")
	name := path[len(path)-1]

	api.QueryCookieName(name, "")
}

func (api *APIRequest) QueryCookieName(name, defQuery string) {
	urlValues := url.Values{}
	cookieValues := url.Values{}

	cookie, err := api.ctx.Request.Cookie(name)
	if err == nil {
		cookieValues, err = url.ParseQuery(cookie.Value)
		if err != nil {
			api.mdw.LogWeb().Warn(api.err.Error())
		}
	} else {
		cookieValues, err = url.ParseQuery(defQuery)
		if err != nil {
			api.mdw.LogWeb().Warn(api.err.Error())
		}
	}

	reqURL := api.ctx.Request.URL
	urlValues, err = url.ParseQuery(reqURL.RawQuery)
	if err != nil {
		api.mdw.LogWeb().Warn(api.err.Error())
	}

	for k, v := range urlValues {
		cookieValues[k] = v
	}

	urlValues = cookieValues
	reqURL.RawQuery = urlValues.Encode()

	skipKeys := []string{"after", "before", "tag", "sort", "section", "query", "to"}
	for _, key := range skipKeys {
		cookieValues.Del(key)
	}

	saveVal := cookieValues.Encode()
	if cookie == nil || saveVal != cookie.Value {
		cookie = &http.Cookie{
			Name:     name,
			Value:    saveVal,
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 90,
			SameSite: http.SameSiteLaxMode,
		}
		api.SetCookie(cookie)
	}
}

func (api *APIRequest) SetQuery(key, value string) string {
	reqURL := api.ctx.Request.URL
	urlValues := reqURL.Query()
	oldValue := urlValues.Get(key)
	urlValues.Set(key, value)
	reqURL.RawQuery = urlValues.Encode()

	return oldValue
}

func (api *APIRequest) SetCookie(cookie *http.Cookie) {
	http.SetCookie(api.ctx.Writer, cookie)
}

func (api *APIRequest) Cookie(name string) (*http.Cookie, error) {
	return api.ctx.Request.Cookie(name)
}

func (api *APIRequest) ClearCookieToken() {
	cookie := &http.Cookie{
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	}

	cookie.Name = "at"
	cookie.Domain = api.mdw.ConfigString("web.domain")
	api.SetCookie(cookie)

	cookie.Name = "trr"
	api.SetCookie(cookie)

	cookie.Name = "trp"
	api.SetCookie(cookie)

	cookie.Name = "rt"
	cookie.Domain = api.mdw.ConfigString("auth.domain")
	api.SetCookie(cookie)
}

func (api *APIRequest) checkError() {
	if api.err != nil || api.resp == nil {
		return
	}

	code := api.resp.StatusCode
	switch {
	case code == 401:
		api.RequestRefreshAuth()
		api.err = http.ErrNoCookie
	case code >= 400 && code < 500:
		if api.err != nil {
			api.mdw.LogWeb().Warn(api.err.Error())
		}
		api.err = clientError
	case code >= 500:
		if api.err != nil {
			api.mdw.LogWeb().Warn(api.err.Error())
		}
		api.err = serverError
	}
}

func (api *APIRequest) copyRequestToHost(path, host string) *http.Request {
	req := api.ctx.Request.Clone(api.ctx.Request.Context())
	req.URL.Scheme = api.mdw.scheme
	req.URL.Host = host
	req.Host = host
	req.URL.Path = api.mdw.path + path
	req.Close = false

	req.Header = make(map[string][]string)
	headers := [...]string{"Accept", "Content-Length", "Content-Type", "Referer", "User-Agent", "X-Forwarded-For"}
	for _, k := range headers {
		vv := api.ctx.Request.Header[k]
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		req.Header[k] = vv2
	}

	req.Header.Set("Authorization", "Bearer "+api.authToken())

	if api.IsGet() && !api.IsAjax() {
		dev, err := api.ctx.Cookie("dev")
		if err == nil {
			req.Header.Set("X-Dev", dev)
		}

		uid, err := api.ctx.Cookie("uid")
		if err == nil {
			req.Header.Set("X-Uid", uid)
		}

		if api.uid2 != "" {
			req.Header.Set("X-Uid2", api.uid2)
		}

		app := requestBrowserID.Build(api.ctx.Request)
		req.Header.Set("X-App", app.String())
	}

	return req
}

func (api *APIRequest) copyRequest(path string) *http.Request {
	return api.copyRequestToHost(path, api.mdw.host)
}

func (api *APIRequest) MethodForwardToHost(method, path, host string, allowNoKey bool) {
	if !api.setUserKey(allowNoKey) {
		api.RequestRefreshAuth()
		return
	}

	req := api.copyRequestToHost(path, host)
	req.Method = method

	api.do(req)
	api.checkError()
}

func (api *APIRequest) MethodForwardToImages(method, path string) {
	api.MethodForwardToHost(method, path, api.mdw.imgHost, false)
}

func (api *APIRequest) MethodForwardToNamed(method, path, name string, allowNoKey bool) {
	if !api.setUserKey(allowNoKey) {
		return
	}

	req := api.copyRequest(path)
	req.Method = method

	api.doNamed(req, name)
	api.checkError()
}

func (api *APIRequest) MethodForwardTo(method, path string, allowNoKey bool) {
	api.MethodForwardToHost(method, path, api.mdw.host, allowNoKey)
}

func (api *APIRequest) MethodForward(method string) {
	api.MethodForwardTo(method, api.ctx.Request.URL.Path, false)
}

func (api *APIRequest) ForwardToAllowNoKey(path string, allowNoKey bool) {
	api.MethodForwardTo(api.ctx.Request.Method, path, allowNoKey)
}

func (api *APIRequest) ForwardToNoKey(path string) {
	api.MethodForwardTo(api.ctx.Request.Method, path, true)
}

func (api *APIRequest) ForwardTo(path string) {
	api.MethodForwardTo(api.ctx.Request.Method, path, false)
}

func (api *APIRequest) ForwardImages() {
	api.MethodForwardToImages(api.ctx.Request.Method, api.ctx.Request.URL.Path)
}

func (api *APIRequest) Forward() {
	api.ForwardTo(api.ctx.Request.URL.Path)
}

func (api *APIRequest) ForwardNoKey() {
	api.ForwardToAllowNoKey(api.ctx.Request.URL.Path, true)
}

func (api *APIRequest) SetFieldAllowNoKey(key, path string, allowNoKey bool) {
	if api.err != nil {
		return
	}

	if api.data == nil {
		api.data = api.parseResponse()
	}

	api.MethodForwardToNamed("GET", path, key, allowNoKey)

	if api.err != nil {
		return
	}

	if api.data == nil {
		api.data = map[string]interface{}{}
	}

	api.data[key] = api.parseResponse()
}

func (api *APIRequest) SetFieldNoKey(key, path string) {
	api.SetFieldAllowNoKey(key, path, true)
}

func (api *APIRequest) SetField(key, path string) {
	api.SetFieldAllowNoKey(key, path, false)
}

func (api *APIRequest) SetMe() {
	api.SetField("me", "/me")
}

func (api *APIRequest) readResponse() []byte {
	if api.resp == nil || api.read {
		return nil
	}

	api.read = true

	var jsonData []byte
	jsonData, api.err = ioutil.ReadAll(api.resp.Body)
	api.resp.Body.Close()
	if api.err != nil {
		api.mdw.LogWeb().Error(api.err.Error())
	}

	return jsonData
}

func (api *APIRequest) parseResponse() map[string]interface{} {
	jsonData := api.readResponse()
	if len(jsonData) == 0 {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewBuffer(jsonData))
	decoder.UseNumber()
	var data map[string]interface{}
	api.err = decoder.Decode(&data)
	if api.err != nil {
		api.mdw.LogWeb().Error(api.err.Error(),
			zap.ByteString("json", jsonData),
		)
	}

	return data
}

func (api *APIRequest) setErrorCode() {
	if api.data == nil {
		api.data = api.parseResponse()
		if api.data == nil {
			api.data = map[string]interface{}{}
		}
	}

	code := api.data["code"]
	if code != nil {
		return
	}

	if api.resp.StatusCode >= 400 {
		code = api.resp.StatusCode
	} else {
		code = 500
	}

	api.data["code"] = code
}

func (api *APIRequest) WriteTemplate(name string) {
	switch api.err {
	case clientError:
		name = "error"
		api.setErrorCode()
		break
	case serverError:
		name = "server_error"
		api.ctx.Status(500)
		break
	case csrfError:
		name = "error"
		api.ctx.Status(419)
		break
	case redirectedErr:
		return
	case nil:
		break
	default:
		return
	}

	if (name == "error" || name == "server_error") && (api.ExpectsJsonError() || !api.IsWebRequest()) {
		api.WriteJson()
		return
	}

	var templ *pongo2.Template
	templ, api.err = api.mdw.Template(name)
	if api.err != nil {
		return
	}

	if api.resp != nil {
		api.ctx.Status(api.resp.StatusCode)
	}

	api.SetData("__large_screen", api.IsLargeScreen())
	api.SetData("__proto", api.mdw.ConfigString("web.proto"))
	api.SetData("__domain", api.mdw.ConfigString("web.domain"))
	api.SetData("__to_url", api.NextRedirect())
	api.SetData("__logged_in", api.HasUserKey())

	authUrl := api.mdw.ConfigString("auth.proto") + "://" + api.mdw.ConfigString("auth.domain")
	api.SetData("__auth_url", authUrl)

	if api.mdw.DevMode {
		api.SetData("__test", true)
	}

	api.ctx.Header("Cache-Control", "no-store")
	api.ctx.Header("Content-Type", "text/html; charset=utf-8")
	api.ctx.Header("Referrer-Policy", "origin")
	api.st.WriteHeader(api.ctx.Writer)

	if _, err := api.ctx.Cookie("uid2"); err != nil && api.uid2 != "" {
		cookie := &http.Cookie{
			Name:     "uid2",
			Value:    api.uid2,
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365 * 5,
			SameSite: http.SameSiteLaxMode,
		}
		api.SetCookie(cookie)
	}

	templ.ExecuteWriter(api.Data(), api.ctx.Writer)
}

func (api *APIRequest) WriteTemplateWithExtension(name string) {
	var templ *pongo2.Template
	templ, api.err = api.mdw.TemplateWithExtension(name)
	if api.err != nil {
		return
	}

	if api.resp != nil {
		api.ctx.Status(api.resp.StatusCode)
	}

	api.SetData("__proto", api.mdw.ConfigString("web.proto"))
	api.SetData("__domain", api.mdw.ConfigString("web.domain"))

	templ.ExecuteWriter(api.Data(), api.ctx.Writer)
}

func (api *APIRequest) WriteResponse() {
	jsonData := api.readResponse()
	if api.resp == nil {
		return
	}

	for k, vv := range api.resp.Header {
		for _, v := range vv {
			api.ctx.Header(k, v)
		}
	}

	api.ctx.Status(api.resp.StatusCode)

	api.ctx.Header("Cache-Control", "no-store")
	api.st.WriteHeader(api.ctx.Writer)

	if jsonData != nil {
		api.ctx.Writer.Write(jsonData)
	}
}

func (api *APIRequest) WriteJson() {
	if api.resp != nil {
		api.ctx.Status(api.resp.StatusCode)
	}

	api.ctx.Header("Cache-Control", "no-store")
	api.ctx.Header("Content-Type", "application/json")
	api.st.WriteHeader(api.ctx.Writer)

	encoder := json.NewEncoder(api.ctx.Writer)
	api.err = encoder.Encode(api.data)
	if api.err != nil {
		api.mdw.LogWeb().Error(api.err.Error())
	}
}

func (api *APIRequest) RedirectToHost(url string) {
	if api.err != nil {
		api.WriteTemplate("error")
		return
	}

	api.ctx.Redirect(http.StatusSeeOther, url)
}

func (api *APIRequest) Redirect(path string) {
	base := api.mdw.ConfigString("web.proto") + "://" + api.mdw.ConfigString("web.domain")
	api.RedirectToHost(base + path)
}

func (api *APIRequest) RedirectToAuth(path string) {
	base := api.mdw.ConfigString("auth.proto") + "://" + api.mdw.ConfigString("auth.domain")
	api.RedirectToHost(base + path)
}

func (api *APIRequest) RedirectQuery(def string) {
	to := api.ctx.Query("to")
	if to == "" {
		to = def
	}

	api.Redirect(to)
}

func (api *APIRequest) NextRedirect() string {
	to := api.ctx.Query("to")
	if to == "" {
		to = api.ctx.Request.URL.String()
	}

	return url.QueryEscape(to)
}

func (api *APIRequest) ClientIP() string {
	return api.Header("X-Forwarded-For")
}

func (api *APIRequest) IsGet() bool {
	return api.ctx.Request.Method == "GET"
}

func (api *APIRequest) IsWebRequest() bool {
	host := api.mdw.ConfigString("web.domain")
	return api.ctx.Request.Host == host
}

func (api *APIRequest) IsAjax() bool {
	with := api.Header("X-Requested-With")
	return with == "XMLHttpRequest"
}

func (api *APIRequest) IsAuthRefreshRequired() bool {
	_, err := api.Cookie("trr")
	return err != nil
}

func (api *APIRequest) IsAuthRefreshPossible() bool {
	_, err := api.Cookie("trp")
	return err == nil
}

func (api *APIRequest) ExpectsJsonError() bool {
	errType := api.Header("X-Error-Type")
	return errType == "JSON"
}

func (api *APIRequest) IsLargeScreen() bool {
	vpw, err := api.ctx.Cookie("vpw")
	if err == nil {
		width, err := strconv.Atoi(vpw)
		if err == nil {
			const bootstrapExtraLargeWidth = 1199
			return width >= bootstrapExtraLargeWidth
		}
	}

	ua := api.ctx.GetHeader("User-Agent")

	if mobReFull.MatchString(ua) {
		return false
	}

	if len(ua) < 4 {
		return true
	}

	return !mobRe4.MatchString(ua[:4])
}

func (api *APIRequest) AppID() string {
	return api.mdw.apiID
}

func (api *APIRequest) AppSecret() string {
	return api.mdw.apiSecret
}
