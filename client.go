package fiber

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/internal/encoding/json"
	"github.com/valyala/fasthttp"
)

// Request represents HTTP request.
//
// It is forbidden copying Request instances. Create new instances
// and use CopyTo instead.
//
// Request instance MUST NOT be used from concurrently running goroutines.
type Request = fasthttp.Request

// Response represents HTTP response.
//
// It is forbidden copying Response instances. Create new instances
// and use CopyTo instead.
//
// Response instance MUST NOT be used from concurrently running goroutines.
type Response = fasthttp.Response

// Args represents query arguments.
//
// It is forbidden copying Args instances. Create new instances instead
// and use CopyTo().
//
// Args instance MUST NOT be used from concurrently running goroutines.
type Args = fasthttp.Args

var defaultClient Client

// Client implements http client.
//
// It is safe calling Client methods from concurrently running goroutines.
type Client struct {
	UserAgent                string
	NoDefaultUserAgentHeader bool
}

// Get returns a agent with http method GET.
func Get(url string) *Agent { return defaultClient.Get(url) }

// Get returns a agent with http method GET.
func (c *Client) Get(url string) *Agent {
	return c.createAgent(MethodGet, url)
}

// Post sends POST request to the given url.
func Post(url string) *Agent { return defaultClient.Post(url) }

// Post sends POST request to the given url.
func (c *Client) Post(url string) *Agent {
	return c.createAgent(MethodPost, url)
}

func (c *Client) createAgent(method, url string) *Agent {
	a := AcquireAgent()
	a.req.Header.SetMethod(method)
	a.req.SetRequestURI(url)

	a.Name = c.UserAgent
	a.NoDefaultUserAgentHeader = c.NoDefaultUserAgentHeader

	if err := a.Parse(); err != nil {
		a.errs = append(a.errs, err)
	}

	return a
}

// Agent is an object storing all request data for client.
type Agent struct {
	*fasthttp.HostClient
	req                      *Request
	customReq                *Request
	args                     *Args
	timeout                  time.Duration
	errs                     []error
	formFiles                []*FormFile
	debugWriter              io.Writer
	maxRedirectsCount        int
	boundary                 string
	Name                     string
	NoDefaultUserAgentHeader bool
	reuse                    bool
	parsed                   bool
}

var ErrorInvalidURI = fasthttp.ErrorInvalidURI

// Parse initializes URI and HostClient.
func (a *Agent) Parse() error {
	if a.parsed {
		return nil
	}
	a.parsed = true

	req := a.req
	if a.customReq != nil {
		req = a.customReq
	}

	uri := req.URI()
	if uri == nil {
		return ErrorInvalidURI
	}

	isTLS := false
	scheme := uri.Scheme()
	if bytes.Equal(scheme, strHTTPS) {
		isTLS = true
	} else if !bytes.Equal(scheme, strHTTP) {
		return fmt.Errorf("unsupported protocol %q. http and https are supported", scheme)
	}

	name := a.Name
	if name == "" && !a.NoDefaultUserAgentHeader {
		name = defaultUserAgent
	}

	a.HostClient = &fasthttp.HostClient{
		Addr:                     addMissingPort(string(uri.Host()), isTLS),
		Name:                     name,
		NoDefaultUserAgentHeader: a.NoDefaultUserAgentHeader,
		IsTLS:                    isTLS,
	}

	return nil
}

func addMissingPort(addr string, isTLS bool) string {
	n := strings.Index(addr, ":")
	if n >= 0 {
		return addr
	}
	port := 80
	if isTLS {
		port = 443
	}
	return net.JoinHostPort(addr, strconv.Itoa(port))
}

/************************** Header Setting **************************/

// Set sets the given 'key: value' header.
//
// Use Add for setting multiple header values under the same key.
func (a *Agent) Set(k, v string) *Agent {
	a.req.Header.Set(k, v)

	return a
}

// Add adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
func (a *Agent) Add(k, v string) *Agent {
	a.req.Header.Add(k, v)

	return a
}

// ConnectionClose sets 'Connection: close' header.
func (a *Agent) ConnectionClose() *Agent {
	a.req.Header.SetConnectionClose()

	return a
}

// UserAgent sets User-Agent header value.
func (a *Agent) UserAgent(userAgent string) *Agent {
	a.req.Header.SetUserAgent(userAgent)

	return a
}

// Cookie sets one 'key: value' cookie.
func (a *Agent) Cookie(key, value string) *Agent {
	a.req.Header.SetCookie(key, value)

	return a
}

// Cookies sets multiple 'key: value' cookies.
func (a *Agent) Cookies(kv ...string) *Agent {
	for i := 1; i < len(kv); i += 2 {
		a.req.Header.SetCookie(kv[i-1], kv[i])
	}

	return a
}

// Referer sets Referer header value.
func (a *Agent) Referer(referer string) *Agent {
	a.req.Header.SetReferer(referer)

	return a
}

// ContentType sets Content-Type header value.
func (a *Agent) ContentType(contentType string) *Agent {
	a.req.Header.SetContentType(contentType)

	return a
}

/************************** End Header Setting **************************/

/************************** URI Setting **************************/

// Host sets host for the uri.
func (a *Agent) Host(host string) *Agent {
	a.req.URI().SetHost(host)

	return a
}

// QueryString sets URI query string.
func (a *Agent) QueryString(queryString string) *Agent {
	a.req.URI().SetQueryString(queryString)

	return a
}

// BasicAuth sets URI username and password.
func (a *Agent) BasicAuth(username, password string) *Agent {
	a.req.URI().SetUsername(username)
	a.req.URI().SetPassword(password)

	return a
}

/************************** End URI Setting **************************/

/************************** Request Setting **************************/

// BodyString sets request body.
func (a *Agent) BodyString(bodyString string) *Agent {
	a.req.SetBodyString(bodyString)

	return a
}

// BodyStream sets request body stream and, optionally body size.
//
// If bodySize is >= 0, then the bodyStream must provide exactly bodySize bytes
// before returning io.EOF.
//
// If bodySize < 0, then bodyStream is read until io.EOF.
//
// bodyStream.Close() is called after finishing reading all body data
// if it implements io.Closer.
//
// Note that GET and HEAD requests cannot have body.
func (a *Agent) BodyStream(bodyStream io.Reader, bodySize int) *Agent {
	a.req.SetBodyStream(bodyStream, bodySize)

	return a
}

// Request sets custom request for createAgent.
func (a *Agent) Request(req *Request) *Agent {
	a.customReq = req

	return a
}

// JSON sends a JSON request.
func (a *Agent) JSON(v interface{}) *Agent {
	a.req.Header.SetContentType(MIMEApplicationJSON)

	if body, err := json.Marshal(v); err != nil {
		a.errs = append(a.errs, err)
	} else {
		a.req.SetBody(body)
	}

	return a
}

// XML sends a XML request.
func (a *Agent) XML(v interface{}) *Agent {
	a.req.Header.SetContentType(MIMEApplicationXML)

	if body, err := xml.Marshal(v); err != nil {
		a.errs = append(a.errs, err)
	} else {
		a.req.SetBody(body)
	}

	return a
}

// Form sends request with body if args is non-nil.
//
// It is recommended obtaining args via AcquireArgs
// in performance-critical code.
func (a *Agent) Form(args *Args) *Agent {
	a.req.Header.SetContentType(MIMEApplicationForm)

	if args != nil {
		a.req.SetBody(args.QueryString())
	}

	return a
}

// FormFile represents multipart form file
type FormFile struct {
	// Fieldname is form file's field name
	Fieldname string
	// Name is form file's name
	Name string
	// Content is form file's content
	Content []byte
	// autoRelease indicates if returns the object
	// acquired via AcquireFormFile to the pool.
	autoRelease bool
}

// FileData appends files for multipart form request.
//
// It is recommended obtaining formFile via AcquireFormFile
// in performance-critical code.
func (a *Agent) FileData(formFiles ...*FormFile) *Agent {
	a.formFiles = append(a.formFiles, formFiles...)

	return a
}

// SendFile reads file and appends it to multipart form request.
func (a *Agent) SendFile(filename string, fieldname ...string) *Agent {
	content, err := ioutil.ReadFile(filepath.Clean(filename))
	if err != nil {
		a.errs = append(a.errs, err)
		return a
	}

	ff := AcquireFormFile()
	if len(fieldname) > 0 && fieldname[0] != "" {
		ff.Fieldname = fieldname[0]
	} else {
		ff.Fieldname = "file" + strconv.Itoa(len(a.formFiles)+1)
	}
	ff.Name = filepath.Base(filename)
	ff.Content = append(ff.Content, content...)
	ff.autoRelease = true

	a.formFiles = append(a.formFiles, ff)

	return a
}

// SendFiles reads files and appends them to multipart form request.
//
// Examples:
//		SendFile("/path/to/file1", "fieldname1", "/path/to/file2")
func (a *Agent) SendFiles(filenamesAndFieldnames ...string) *Agent {
	pairs := len(filenamesAndFieldnames)
	if pairs&1 == 1 {
		filenamesAndFieldnames = append(filenamesAndFieldnames, "")
	}

	for i := 0; i < pairs; i += 2 {
		a.SendFile(filenamesAndFieldnames[i], filenamesAndFieldnames[i+1])
	}

	return a
}

// Boundary sets boundary for multipart form request.
func (a *Agent) Boundary(boundary string) *Agent {
	a.boundary = boundary

	return a
}

// MultipartForm sends multipart form request with k-v and files.
//
// It is recommended obtaining args via AcquireArgs
// in performance-critical code.
func (a *Agent) MultipartForm(args *Args) *Agent {
	mw := multipart.NewWriter(a.req.BodyWriter())

	if a.boundary != "" {
		if err := mw.SetBoundary(a.boundary); err != nil {
			a.errs = append(a.errs, err)
			return a
		}
	}

	a.req.Header.SetMultipartFormBoundary(mw.Boundary())

	if args != nil {
		args.VisitAll(func(key, value []byte) {
			if err := mw.WriteField(getString(key), getString(value)); err != nil {
				a.errs = append(a.errs, err)
			}
		})
	}

	for _, ff := range a.formFiles {
		w, err := mw.CreateFormFile(ff.Fieldname, ff.Name)
		if err != nil {
			a.errs = append(a.errs, err)
			continue
		}
		if _, err = w.Write(ff.Content); err != nil {
			a.errs = append(a.errs, err)
		}
	}

	if err := mw.Close(); err != nil {
		a.errs = append(a.errs, err)
	}

	return a
}

/************************** End Request Setting **************************/

/************************** Agent Setting **************************/

// Debug mode enables logging request and response detail
func (a *Agent) Debug(w ...io.Writer) *Agent {
	a.debugWriter = os.Stdout
	if len(w) > 0 {
		a.debugWriter = w[0]
	}

	return a
}

// Timeout sets request timeout duration.
func (a *Agent) Timeout(timeout time.Duration) *Agent {
	a.timeout = timeout

	return a
}

// Reuse indicates the createAgent can be used again after one request.
func (a *Agent) Reuse() *Agent {
	a.reuse = true

	return a
}

// InsecureSkipVerify controls whether the createAgent verifies the server's
// certificate chain and host name.
func (a *Agent) InsecureSkipVerify() *Agent {
	if a.HostClient.TLSConfig == nil {
		/* #nosec G402 */
		a.HostClient.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		/* #nosec G402 */
		a.HostClient.TLSConfig.InsecureSkipVerify = true
	}

	return a
}

// TLSConfig sets tls config.
func (a *Agent) TLSConfig(config *tls.Config) *Agent {
	a.HostClient.TLSConfig = config

	return a
}

// MaxRedirectsCount sets max redirect count for GET and HEAD.
func (a *Agent) MaxRedirectsCount(count int) *Agent {
	a.maxRedirectsCount = count

	return a
}

/************************** End Agent Setting **************************/

// Bytes returns the status code, bytes body and errors of url.
func (a *Agent) Bytes(customResp ...*Response) (code int, body []byte, errs []error) {
	defer a.release()

	if errs = append(errs, a.errs...); len(errs) > 0 {
		return
	}

	req := a.req
	if a.customReq != nil {
		req = a.customReq
	}

	var (
		resp        *Response
		releaseResp bool
	)
	if len(customResp) > 0 {
		resp = customResp[0]
	} else {
		resp = AcquireResponse()
		releaseResp = true
	}
	defer func() {
		if a.debugWriter != nil {
			printDebugInfo(req, resp, a.debugWriter)
		}

		if len(errs) == 0 {
			code = resp.StatusCode()
		}

		if releaseResp {
			body = append(body, resp.Body()...)
			ReleaseResponse(resp)
		} else {
			body = resp.Body()
		}
	}()

	if a.timeout > 0 {
		if err := a.HostClient.DoTimeout(req, resp, a.timeout); err != nil {
			errs = append(errs, err)
			return
		}
	}

	if a.maxRedirectsCount > 0 && (string(req.Header.Method()) == MethodGet || string(req.Header.Method()) == MethodHead) {
		if err := a.HostClient.DoRedirects(req, resp, a.maxRedirectsCount); err != nil {
			errs = append(errs, err)
			return
		}
	}

	if err := a.HostClient.Do(req, resp); err != nil {
		errs = append(errs, err)
	}

	return
}

func printDebugInfo(req *Request, resp *Response, w io.Writer) {
	msg := fmt.Sprintf("Connected to %s(%s)\r\n\r\n", req.URI().Host(), resp.RemoteAddr())
	_, _ = w.Write(getBytes(msg))
	_, _ = req.WriteTo(w)
	_, _ = resp.WriteTo(w)
}

// String returns the status code, string body and errors of url.
func (a *Agent) String(resp ...*Response) (int, string, []error) {
	code, body, errs := a.Bytes(resp...)

	return code, getString(body), errs
}

// Struct returns the status code, bytes body and errors of url.
// And bytes body will be unmarshalled to given v.
func (a *Agent) Struct(v interface{}, resp ...*Response) (code int, body []byte, errs []error) {
	code, body, errs = a.Bytes(resp...)

	if err := json.Unmarshal(body, v); err != nil {
		errs = append(errs, err)
	}

	return
}

func (a *Agent) release() {
	if !a.reuse {
		ReleaseAgent(a)
	} else {
		a.errs = a.errs[:0]
	}
}

func (a *Agent) reset() {
	a.HostClient = nil
	a.req.Reset()
	a.customReq = nil
	a.timeout = 0
	a.args = nil
	a.errs = a.errs[:0]
	a.debugWriter = nil
	a.reuse = false
	a.parsed = false
	a.maxRedirectsCount = 0
	a.boundary = ""
	a.Name = ""
	a.NoDefaultUserAgentHeader = false
	for i, ff := range a.formFiles {
		if ff.autoRelease {
			ReleaseFormFile(ff)
		}
		a.formFiles[i] = nil
	}
	a.formFiles = a.formFiles[:0]
}

var (
	clientPool   sync.Pool
	agentPool    sync.Pool
	requestPool  sync.Pool
	responsePool sync.Pool
	argsPool     sync.Pool
	formFilePool sync.Pool
)

// AcquireAgent returns an empty Agent instance from createAgent pool.
//
// The returned Agent instance may be passed to ReleaseAgent when it is
// no longer needed. This allows Agent recycling, reduces GC pressure
// and usually improves performance.
func AcquireAgent() *Agent {
	v := agentPool.Get()
	if v == nil {
		return &Agent{req: fasthttp.AcquireRequest()}
	}
	return v.(*Agent)
}

// ReleaseAgent returns a acquired via AcquireAgent to createAgent pool.
//
// It is forbidden accessing req and/or its' members after returning
// it to createAgent pool.
func ReleaseAgent(a *Agent) {
	a.reset()
	agentPool.Put(a)
}

// AcquireClient returns an empty Client instance from client pool.
//
// The returned Client instance may be passed to ReleaseClient when it is
// no longer needed. This allows Client recycling, reduces GC pressure
// and usually improves performance.
func AcquireClient() *Client {
	v := clientPool.Get()
	if v == nil {
		return &Client{}
	}
	return v.(*Client)
}

// ReleaseClient returns c acquired via AcquireClient to client pool.
//
// It is forbidden accessing req and/or its' members after returning
// it to client pool.
func ReleaseClient(c *Client) {
	c.UserAgent = ""
	c.NoDefaultUserAgentHeader = false

	clientPool.Put(c)
}

// AcquireRequest returns an empty Request instance from request pool.
//
// The returned Request instance may be passed to ReleaseRequest when it is
// no longer needed. This allows Request recycling, reduces GC pressure
// and usually improves performance.
func AcquireRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		return &Request{}
	}
	return v.(*Request)
}

// ReleaseRequest returns req acquired via AcquireRequest to request pool.
//
// It is forbidden accessing req and/or its' members after returning
// it to request pool.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}

// AcquireResponse returns an empty Response instance from response pool.
//
// The returned Response instance may be passed to ReleaseResponse when it is
// no longer needed. This allows Response recycling, reduces GC pressure
// and usually improves performance.
// Copy from fasthttp
func AcquireResponse() *Response {
	v := responsePool.Get()
	if v == nil {
		return &Response{}
	}
	return v.(*Response)
}

// ReleaseResponse return resp acquired via AcquireResponse to response pool.
//
// It is forbidden accessing resp and/or its' members after returning
// it to response pool.
// Copy from fasthttp
func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}

// AcquireArgs returns an empty Args object from the pool.
//
// The returned Args may be returned to the pool with ReleaseArgs
// when no longer needed. This allows reducing GC load.
// Copy from fasthttp
func AcquireArgs() *Args {
	v := argsPool.Get()
	if v == nil {
		return &Args{}
	}
	return v.(*Args)
}

// ReleaseArgs returns the object acquired via AcquireArgs to the pool.
//
// String not access the released Args object, otherwise data races may occur.
// Copy from fasthttp
func ReleaseArgs(a *Args) {
	a.Reset()
	argsPool.Put(a)
}

// AcquireFormFile returns an empty FormFile object from the pool.
//
// The returned FormFile may be returned to the pool with ReleaseFormFile
// when no longer needed. This allows reducing GC load.
func AcquireFormFile() *FormFile {
	v := formFilePool.Get()
	if v == nil {
		return &FormFile{}
	}
	return v.(*FormFile)
}

// ReleaseFormFile returns the object acquired via AcquireFormFile to the pool.
//
// String not access the released FormFile object, otherwise data races may occur.
func ReleaseFormFile(ff *FormFile) {
	ff.Fieldname = ""
	ff.Name = ""
	ff.Content = ff.Content[:0]
	ff.autoRelease = false

	formFilePool.Put(ff)
}

var (
	strHTTP          = []byte("http")
	strHTTPS         = []byte("https")
	defaultUserAgent = "fiber"
)
