package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/process"
	"github.com/dop251/goja_nodejs/require"
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/mailer"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/subscriptions"
	pbtemplate "github.com/pocketbase/pocketbase/tools/template"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/spf13/cast"
)

func bindAutomationCodeStepJsvmGlobals(vm *goja.Runtime, app App) error {
	new(require.Registry).Enable(vm)
	console.Enable(vm)
	process.Enable(vm)
	buffer.Enable(vm)

	bindAutomationCodeStepCore(vm)
	bindAutomationCodeStepDbx(vm)
	bindAutomationCodeStepSecurity(vm)
	bindAutomationCodeStepOS(vm)
	bindAutomationCodeStepFilepath(vm)
	bindAutomationCodeStepHTTP(vm)
	bindAutomationCodeStepFilesystem(vm)

	if err := vm.Set("$app", app); err != nil {
		return fmt.Errorf("failed to initialize code step $app: %w", err)
	}
	if err := vm.Set("$template", pbtemplate.NewRegistry()); err != nil {
		return fmt.Errorf("failed to initialize code step $template: %w", err)
	}

	return nil
}

func bindAutomationCodeStepCore(vm *goja.Runtime) {
	vm.SetFieldNameMapper(automationCodeStepFieldMapper{})

	vm.Set("readerToString", func(r io.Reader, maxBytes int) (string, error) {
		return automationCodeStepReaderToString(r, maxBytes)
	})
	vm.Set("toBytes", automationCodeStepToBytes)
	vm.Set("toString", func(raw any, maxReaderBytes int) (string, error) {
		switch v := raw.(type) {
		case io.Reader:
			return automationCodeStepReaderToString(v, maxReaderBytes)
		default:
			str, err := cast.ToStringE(v)
			if err == nil {
				return str, nil
			}
			rawBytes, _ := json.Marshal(raw)
			return string(rawBytes), nil
		}
	})
	vm.Set("sleep", func(milliseconds int64) {
		time.Sleep(time.Duration(milliseconds) * time.Millisecond)
	})
	vm.Set("unmarshal", func(data, dst any) error {
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, &dst)
	})
	vm.Set("Record", func(call goja.ConstructorCall) *goja.Object {
		var instance *Record
		if collection, ok := call.Argument(0).Export().(*Collection); ok {
			instance = NewRecord(collection)
			if data, ok := call.Argument(1).Export().(map[string]any); ok {
				instance.Load(data)
			}
		} else {
			instance = &Record{}
		}
		value := vm.ToValue(instance).(*goja.Object)
		value.SetPrototype(call.This.Prototype())
		return value
	})
	vm.Set("Collection", func(call goja.ConstructorCall) *goja.Object {
		return automationCodeStepStructConstructorUnmarshal(vm, call, &Collection{})
	})
	vm.Set("FieldsList", func(call goja.ConstructorCall) *goja.Object {
		return automationCodeStepStructConstructorUnmarshal(vm, call, &FieldsList{})
	})
	vm.Set("MailerMessage", func(call goja.ConstructorCall) *goja.Object {
		return automationCodeStepStructConstructor(vm, call, &mailer.Message{})
	})
	vm.Set("RequestInfo", func(call goja.ConstructorCall) *goja.Object {
		return automationCodeStepStructConstructor(vm, call, &RequestInfo{Context: RequestInfoContextDefault})
	})
	vm.Set("ValidationError", func(call goja.ConstructorCall) *goja.Object {
		code, _ := call.Argument(0).Export().(string)
		message, _ := call.Argument(1).Export().(string)
		value := vm.ToValue(validation.NewError(code, message)).(*goja.Object)
		value.SetPrototype(call.This.Prototype())
		return value
	})
	vm.Set("Cookie", func(call goja.ConstructorCall) *goja.Object {
		return automationCodeStepStructConstructor(vm, call, &http.Cookie{})
	})
	vm.Set("SubscriptionMessage", func(call goja.ConstructorCall) *goja.Object {
		return automationCodeStepStructConstructor(vm, call, &subscriptions.Message{})
	})
	vm.Set("Context", func(call goja.ConstructorCall) *goja.Object {
		var instance context.Context
		if oldCtx, ok := call.Argument(0).Export().(context.Context); ok {
			instance = oldCtx
		} else {
			instance = context.Background()
		}
		if key := call.Argument(1).Export(); key != nil {
			instance = context.WithValue(instance, key, call.Argument(2).Export())
		}
		value := vm.ToValue(instance).(*goja.Object)
		value.SetPrototype(call.This.Prototype())
		return value
	})
	vm.Set("DateTime", func(call goja.ConstructorCall) *goja.Object {
		instance := types.NowDateTime()
		rawDate, _ := call.Argument(0).Export().(string)
		locName, _ := call.Argument(1).Export().(string)
		if rawDate != "" && locName != "" {
			loc, err := time.LoadLocation(locName)
			if err != nil {
				loc = time.UTC
			}
			instance, _ = types.ParseDateTime(cast.ToTimeInDefaultLocation(rawDate, loc))
		} else if rawDate != "" {
			instance, _ = types.ParseDateTime(rawDate)
		}
		value := vm.ToValue(instance).(*goja.Object)
		value.SetPrototype(call.This.Prototype())
		return value
	})
}

func bindAutomationCodeStepDbx(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$dbx", obj)

	obj.Set("exp", dbx.NewExp)
	obj.Set("hashExp", func(data map[string]any) dbx.HashExp { return dbx.HashExp(data) })
	obj.Set("not", dbx.Not)
	obj.Set("and", dbx.And)
	obj.Set("or", dbx.Or)
	obj.Set("in", dbx.In)
	obj.Set("notIn", dbx.NotIn)
	obj.Set("like", dbx.Like)
	obj.Set("orLike", dbx.OrLike)
	obj.Set("notLike", dbx.NotLike)
	obj.Set("orNotLike", dbx.OrNotLike)
	obj.Set("exists", dbx.Exists)
	obj.Set("notExists", dbx.NotExists)
	obj.Set("between", dbx.Between)
	obj.Set("notBetween", dbx.NotBetween)
}

func bindAutomationCodeStepSecurity(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$security", obj)

	obj.Set("md5", security.MD5)
	obj.Set("sha256", security.SHA256)
	obj.Set("sha512", security.SHA512)
	obj.Set("hs256", security.HS256)
	obj.Set("hs512", security.HS512)
	obj.Set("equal", security.Equal)
	obj.Set("randomString", security.RandomString)
	obj.Set("randomStringByRegex", security.RandomStringByRegex)
	obj.Set("randomStringWithAlphabet", security.RandomStringWithAlphabet)
	obj.Set("pseudorandomString", security.PseudorandomString)
	obj.Set("pseudorandomStringWithAlphabet", security.PseudorandomStringWithAlphabet)
	obj.Set("parseUnverifiedJWT", func(token string) (map[string]any, error) {
		return security.ParseUnverifiedJWT(token)
	})
	obj.Set("parseJWT", func(token string, verificationKey string) (map[string]any, error) {
		return security.ParseJWT(token, verificationKey)
	})
	obj.Set("createJWT", func(payload jwt.MapClaims, signingKey string, secDuration int) (string, error) {
		return security.NewJWT(payload, signingKey, time.Duration(secDuration)*time.Second)
	})
	obj.Set("encrypt", security.Encrypt)
	obj.Set("decrypt", func(cipherText, key string) (string, error) {
		result, err := security.Decrypt(cipherText, key)
		if err != nil {
			return "", err
		}
		return string(result), nil
	})
}

func bindAutomationCodeStepFilesystem(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$filesystem", obj)

	obj.Set("s3", filesystem.NewS3)
	obj.Set("local", filesystem.NewLocal)
	obj.Set("fileFromPath", filesystem.NewFileFromPath)
	obj.Set("fileFromBytes", filesystem.NewFileFromBytes)
	obj.Set("fileFromMultipart", filesystem.NewFileFromMultipart)
	obj.Set("fileFromURL", func(url string, secTimeout int) (*filesystem.File, error) {
		if secTimeout == 0 {
			secTimeout = 120
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(secTimeout)*time.Second)
		defer cancel()
		return filesystem.NewFileFromURL(ctx, url)
	})
}

func bindAutomationCodeStepFilepath(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$filepath", obj)

	obj.Set("base", filepath.Base)
	obj.Set("clean", filepath.Clean)
	obj.Set("dir", filepath.Dir)
	obj.Set("ext", filepath.Ext)
	obj.Set("fromSlash", filepath.FromSlash)
	obj.Set("glob", filepath.Glob)
	obj.Set("isAbs", filepath.IsAbs)
	obj.Set("join", filepath.Join)
	obj.Set("match", filepath.Match)
	obj.Set("rel", filepath.Rel)
	obj.Set("split", filepath.Split)
	obj.Set("splitList", filepath.SplitList)
	obj.Set("toSlash", filepath.ToSlash)
	obj.Set("walk", filepath.Walk)
	obj.Set("walkDir", filepath.WalkDir)
}

func bindAutomationCodeStepOS(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$os", obj)

	obj.Set("args", os.Args)
	obj.Set("exec", exec.Command)
	obj.Set("cmd", exec.Command)
	obj.Set("exit", os.Exit)
	obj.Set("getenv", os.Getenv)
	obj.Set("dirFS", os.DirFS)
	obj.Set("stat", os.Stat)
	obj.Set("readFile", os.ReadFile)
	obj.Set("writeFile", os.WriteFile)
	obj.Set("readDir", os.ReadDir)
	obj.Set("tempDir", os.TempDir)
	obj.Set("truncate", os.Truncate)
	obj.Set("getwd", os.Getwd)
	obj.Set("mkdir", os.Mkdir)
	obj.Set("mkdirAll", os.MkdirAll)
	obj.Set("rename", os.Rename)
	obj.Set("remove", os.Remove)
	obj.Set("removeAll", os.RemoveAll)
	obj.Set("openRoot", os.OpenRoot)
	obj.Set("openInRoot", os.OpenInRoot)
}

func bindAutomationCodeStepHTTP(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$http", obj)

	vm.Set("FormData", func(call goja.ConstructorCall) *goja.Object {
		value := vm.ToValue(automationCodeStepFormData{}).(*goja.Object)
		value.SetPrototype(call.This.Prototype())
		return value
	})

	obj.Set("send", func(params map[string]any) (*automationCodeStepHTTPSendResult, error) {
		config := automationCodeStepHTTPSendConfig{Method: "GET"}
		if v, ok := params["data"]; ok {
			config.Data = cast.ToStringMap(v)
		}
		if v, ok := params["body"]; ok {
			config.Body = v
		}
		if v, ok := params["headers"]; ok {
			config.Headers = cast.ToStringMapString(v)
		}
		if v, ok := params["method"]; ok {
			config.Method = cast.ToString(v)
		}
		if v, ok := params["url"]; ok {
			config.URL = cast.ToString(v)
		}
		if v, ok := params["timeout"]; ok {
			config.Timeout = cast.ToInt(v)
		}
		if config.Timeout <= 0 {
			config.Timeout = 120
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Timeout)*time.Second)
		defer cancel()

		reqBody, contentType, err := automationCodeStepHTTPRequestBody(config)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, strings.ToUpper(config.Method), config.URL, reqBody)
		if err != nil {
			return nil, err
		}
		for k, v := range config.Headers {
			req.Header.Add(k, v)
		}
		if contentType != "" {
			req.Header.Set("content-type", contentType)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		bodyRaw, _ := io.ReadAll(res.Body)
		result := &automationCodeStepHTTPSendResult{
			StatusCode: res.StatusCode,
			Headers:    map[string][]string{},
			Cookies:    map[string]*http.Cookie{},
			Raw:        string(bodyRaw),
			Body:       bodyRaw,
		}
		for k, v := range res.Header {
			result.Headers[k] = v
		}
		for _, v := range res.Cookies() {
			result.Cookies[v.Name] = v
		}
		if len(result.Body) > 0 {
			result.JSON = map[string]any{}
			if err := json.Unmarshal(bodyRaw, &result.JSON); err != nil {
				result.JSON = []any{}
				if err := json.Unmarshal(bodyRaw, &result.JSON); err != nil {
					result.JSON = nil
				}
			}
		}

		return result, nil
	})
}

func automationCodeStepHTTPRequestBody(config automationCodeStepHTTPSendConfig) (io.Reader, string, error) {
	if len(config.Data) != 0 {
		encoded, err := json.Marshal(config.Data)
		if err != nil {
			return nil, "", err
		}
		return bytes.NewReader(encoded), "", nil
	}

	switch v := config.Body.(type) {
	case io.Reader:
		return v, "", nil
	case automationCodeStepFormData:
		body, mp, err := v.toMultipart()
		if err != nil {
			return nil, "", err
		}
		return body, mp.FormDataContentType(), nil
	default:
		return strings.NewReader(cast.ToString(config.Body)), "", nil
	}
}

type automationCodeStepHTTPSendResult struct {
	JSON       any                     `json:"json"`
	Headers    map[string][]string     `json:"headers"`
	Cookies    map[string]*http.Cookie `json:"cookies"`
	Raw        string                  `json:"raw"`
	Body       []byte                  `json:"body"`
	StatusCode int                     `json:"statusCode"`
}

type automationCodeStepHTTPSendConfig struct {
	Data    map[string]any
	Body    any
	Headers map[string]string
	Method  string
	URL     string
	Timeout int
}

type automationCodeStepFormData map[string][]any

func (data automationCodeStepFormData) Append(key string, value any) {
	data[key] = append(data[key], value)
}

func (data automationCodeStepFormData) Set(key string, value any) {
	data[key] = []any{value}
}

func (data automationCodeStepFormData) Delete(key string) {
	delete(data, key)
}

func (data automationCodeStepFormData) Get(key string) any {
	values, ok := data[key]
	if !ok || len(values) == 0 {
		return nil
	}
	return values[0]
}

func (data automationCodeStepFormData) GetAll(key string) []any {
	return data[key]
}

func (data automationCodeStepFormData) Has(key string) bool {
	return len(data[key]) > 0
}

func (data automationCodeStepFormData) Keys() []string {
	result := make([]string, 0, len(data))
	for k := range data {
		result = append(result, k)
	}
	return result
}

func (data automationCodeStepFormData) Values() []any {
	result := make([]any, 0, len(data))
	for _, values := range data {
		result = append(result, values...)
	}
	return result
}

func (data automationCodeStepFormData) Entries() [][]any {
	result := make([][]any, 0, len(data))
	for k, values := range data {
		for _, v := range values {
			result = append(result, []any{k, v})
		}
	}
	return result
}

func (data automationCodeStepFormData) toMultipart() (*bytes.Buffer, *multipart.Writer, error) {
	body := new(bytes.Buffer)
	mp := multipart.NewWriter(body)
	defer mp.Close()

	for k, values := range data {
		for _, rawValue := range values {
			if file, ok := rawValue.(*filesystem.File); ok {
				err := func() error {
					mpw, err := mp.CreateFormFile(k, file.OriginalName)
					if err != nil {
						return err
					}
					reader, err := file.Reader.Open()
					if err != nil {
						return err
					}
					defer reader.Close()
					_, err = io.Copy(mpw, reader)
					return err
				}()
				if err != nil {
					return nil, nil, err
				}
				continue
			}
			if err := mp.WriteField(k, cast.ToString(rawValue)); err != nil {
				return nil, nil, err
			}
		}
	}

	return body, mp, nil
}

func automationCodeStepToBytes(raw any, maxReaderBytes int) ([]byte, error) {
	switch v := raw.(type) {
	case nil:
		return []byte{}, nil
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	case types.JSONRaw:
		return v, nil
	case io.Reader:
		if maxReaderBytes == 0 {
			maxReaderBytes = router.DefaultMaxMemory
		}
		return io.ReadAll(io.LimitReader(v, int64(maxReaderBytes)))
	default:
		b, err := cast.ToUint8SliceE(v)
		if err == nil {
			return b, nil
		}
		str, err := cast.ToStringE(v)
		if err == nil {
			return []byte(str), nil
		}
		rawBytes, _ := json.Marshal(raw)
		return rawBytes, nil
	}
}

func automationCodeStepReaderToString(r io.Reader, maxBytes int) (string, error) {
	if maxBytes == 0 {
		maxBytes = router.DefaultMaxMemory
	}
	bodyBytes, readErr := io.ReadAll(io.LimitReader(r, int64(maxBytes)))
	if readErr != nil {
		return "", readErr
	}
	return string(bodyBytes), nil
}

func automationCodeStepStructConstructor(vm *goja.Runtime, call goja.ConstructorCall, instance any) *goja.Object {
	data, _ := call.Argument(0).Export().(map[string]any)
	value := vm.ToValue(instance).(*goja.Object)
	for k, v := range data {
		value.Set(k, v)
	}
	value.SetPrototype(call.This.Prototype())
	return value
}

func automationCodeStepStructConstructorUnmarshal(vm *goja.Runtime, call goja.ConstructorCall, instance any) *goja.Object {
	if data := call.Argument(0).Export(); data != nil {
		if raw, err := json.Marshal(data); err == nil {
			_ = json.Unmarshal(raw, instance)
		}
	}
	value := vm.ToValue(instance).(*goja.Object)
	value.SetPrototype(call.This.Prototype())
	return value
}

func automationCodeStepRegisterFactoryAsConstructor(vm *goja.Runtime, constructorName string, factoryFunc any) {
	rv := reflect.ValueOf(factoryFunc)
	rt := reflect.TypeOf(factoryFunc)
	totalArgs := rt.NumIn()

	vm.Set(constructorName, func(call goja.ConstructorCall) *goja.Object {
		args := make([]reflect.Value, totalArgs)
		for i := 0; i < totalArgs; i++ {
			v := call.Argument(i).Export()
			if v == nil {
				args[i] = reflect.New(rt.In(i)).Elem()
			} else if number, ok := v.(int64); ok {
				args[i] = reflect.ValueOf(int(number))
			} else {
				args[i] = reflect.ValueOf(v)
			}
		}
		result := rv.Call(args)
		if len(result) != 1 {
			panic("the factory function should return only 1 item")
		}
		value := vm.ToValue(result[0].Interface()).(*goja.Object)
		value.SetPrototype(call.This.Prototype())
		return value
	})
}

func normalizeAutomationCodeStepException(err error) error {
	if err == nil {
		return nil
	}

	jsException, ok := err.(*goja.Exception)
	if !ok {
		return err
	}

	switch v := jsException.Value().Export().(type) {
	case error:
		err = v
	case map[string]any:
		if vErr, ok := v["value"].(error); ok {
			err = vErr
		}
	}

	return err
}

type automationCodeStepFieldMapper struct{}

func (automationCodeStepFieldMapper) FieldName(_ reflect.Type, f reflect.StructField) string {
	return convertAutomationCodeStepGoName(f.Name)
}

func (automationCodeStepFieldMapper) MethodName(_ reflect.Type, m reflect.Method) string {
	return convertAutomationCodeStepGoName(m.Name)
}

func convertAutomationCodeStepGoName(name string) string {
	if name == "OAuth2" {
		return "oauth2"
	}

	startUppercase := make([]rune, 0, len(name))
	for _, c := range name {
		if c != '_' && !unicode.IsUpper(c) && !unicode.IsDigit(c) {
			break
		}
		startUppercase = append(startUppercase, c)
	}

	totalStartUppercase := len(startUppercase)
	if len(name) == totalStartUppercase {
		return strings.ToLower(name)
	}
	if totalStartUppercase > 1 {
		return strings.ToLower(name[0:totalStartUppercase-1]) + name[totalStartUppercase-1:]
	}
	if totalStartUppercase == 1 {
		return strings.ToLower(name[0:1]) + name[1:]
	}
	return name
}

func checkAutomationCodeStepValueForError(app App, value goja.Value) error {
	if value == nil {
		return nil
	}

	exported := value.Export()
	switch v := exported.(type) {
	case error:
		return v
	case *goja.Promise:
		app.Logger().Warn("the automation code step must not return a Promise")
		if promiseErr, ok := v.Result().Export().(error); ok {
			return normalizeAutomationCodeStepException(promiseErr)
		}
	}

	return nil
}

var _ goja.FieldNameMapper = (*automationCodeStepFieldMapper)(nil)
