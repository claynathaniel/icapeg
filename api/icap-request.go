package api

import (
	"bytes"
	"errors"
	"icapeg/config"
	"icapeg/consts"
	"icapeg/http-message"
	"icapeg/icap"
	"icapeg/logging"
	"icapeg/service"
	"icapeg/service/services-utilities/ContentTypes"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
)

// ICAPRequest struct is used to encapsulate important information of the ICAP request like method name, etc
type ICAPRequest struct {
	w                      icap.ResponseWriter
	req                    *icap.Request
	h                      http.Header
	Is204Allowed           bool
	isShadowServiceEnabled bool
	appCfg                 *config.AppConfig
	serviceName            string
	methodName             string
	vendor                 string
}

// NewICAPRequest is a func to create a new instance from struct IcapRequest yo handle upcoming ICAP requests
func NewICAPRequest(w icap.ResponseWriter, req *icap.Request) *ICAPRequest {
	ICAPRequest := &ICAPRequest{
		w:      w,
		req:    req,
		h:      w.Header(),
		appCfg: config.App(),
	}
	for serviceName, serviceInstance := range ICAPRequest.appCfg.ServicesInstances {
		service.InitServiceConfig(serviceInstance.Vendor, serviceName)
	}
	return ICAPRequest
}

// RequestInitialization is a fun to retrieve the important information from the ICAP request
// and initialize the ICAP response
func (i *ICAPRequest) RequestInitialization() error {
	logging.Logger.Info("Validating the received ICAP request")
	logging.Logger.Debug("Creating an instance from ICAPeg configuration")
	i.appCfg = config.App()

	//adding headers to the log
	i.addHeadersToLogs()

	// checking if the service doesn't exist in toml file
	// if it does not exist, the response will be 404 ICAP Service Not Found
	logging.Logger.Debug("checking if the service doesn't exist in toml file")
	i.serviceName = i.req.URL.Path[1:len(i.req.URL.Path)]
	if !i.isServiceExists() {
		i.w.WriteHeader(utils.ICAPServiceNotFoundCodeStr, nil, false)
		err := errors.New("service doesn't exist")
		logging.Logger.Error(err.Error())
		return err
	}

	// checking if request method is allowed or not
	logging.Logger.Debug("checking if request method is allowed or not")
	i.methodName = i.req.Method
	if i.methodName != "options" {
		if !i.isMethodAllowed() {
			i.w.WriteHeader(utils.MethodNotAllowedForServiceCodeStr, nil, false)
			err := errors.New("method is not allowed")
			logging.Logger.Error(err.Error())
			return err
		}
		i.methodName = i.req.Method
	}

	//getting vendor name which depends on the name of the service
	i.vendor = i.getVendorName()

	//adding important headers to options ICAP response
	requiredService := service.GetService(i.vendor, i.serviceName, i.methodName,
		&http_message.HttpMsg{Request: i.req.Request, Response: i.req.Response})
	logging.Logger.Debug("adding ISTAG Service Headers")
	i.addingISTAGServiceHeaders(requiredService.ISTagValue())

	logging.Logger.Debug("checking if returning 24 to ICAP client is allowed or not")
	i.Is204Allowed = i.is204Allowed()

	i.isShadowServiceEnabled = config.AppCfg.ServicesInstances[i.serviceName].ShadowService

	//checking if the shadow service is enabled or not to apply shadow service mode
	logging.Logger.Debug("checking if the shadow service is enabled or not to apply shadow service mode")
	if i.isShadowServiceEnabled && i.methodName != "OPTIONS" {
		logging.Logger.Debug("shadow service mode i on")
		i.shadowService()
		go i.RequestProcessing()
		return errors.New("shadow service")
	} else {
		if i.appCfg.DebuggingHeaders {
			logging.Logger.Debug("adding header to ICAP response in OPTIONS mode indicates that shadow service is off")
			i.h["X-ICAPeg-Shadow-Service"] = []string{"false"}
		}
	}

	return nil
}

// RequestProcessing is a func to process the ICAP request upon the service and method required
func (i *ICAPRequest) RequestProcessing() {
	logging.Logger.Info("processing ICAP request upon the service and method required")
	partial := false

	if i.methodName != utils.ICAPModeOptions {
		file := &bytes.Buffer{}
		fileLen := 0
		if i.methodName == utils.ICAPModeResp {
			io.Copy(file, i.req.Response.Body)
			fileLen = file.Len()
			i.req.Response.Header.Set(utils.ContentLength, strconv.Itoa(len(file.Bytes())))
			i.req.Response.Body = io.NopCloser(bytes.NewBuffer(file.Bytes()))
		} else {
			reqContentType := ContentTypes.GetContentType(i.req.Request)
			// getting the file from request and store it in buf as a type of bytes.Buffer
			file = reqContentType.GetFileFromRequest()
			fileLen = file.Len()
			fileBytes := []byte(reqContentType.BodyAfterScanning(file.Bytes()))
			i.req.Request.Header.Set(utils.ContentLength, strconv.Itoa(len(fileBytes)))
			i.req.Request.Body = io.NopCloser(bytes.NewBuffer(fileBytes))
		}
		if fileLen == 0 {
			partial = false

		} else {
			if i.req.Header.Get("Preview") != "" && i.req.EndIndicator != "0; ieof" {
				partial = true
			}
		}
		if i.req.Header.Get("Preview") != "" && i.req.EndIndicator != "0; ieof" {
			partial = true
		}
	}

	i.HostHeader()

	// check the method name
	switch i.methodName {
	// for options mode
	case utils.ICAPModeOptions:
		logging.Logger.Debug("OPTIONS mode")
		i.optionsMode(i.serviceName)
		break

	//for reqmod and respmod
	default:
		logging.Logger.Debug("Response or Request mode")
		i.RespAndReqMods(partial)
	}

}

func (i *ICAPRequest) HostHeader() {
	if i.methodName == "REQMOD" {
		i.req.Request.Header.Set("Host", i.req.Request.Host)
	}
}

func (i *ICAPRequest) RespAndReqMods(partial bool) {
	if i.methodName == utils.ICAPModeReq {
		defer i.req.Request.Body.Close()
	} else {
		defer i.req.Response.Body.Close()
	}
	if i.req.Request == nil {
		i.req.Request = &http.Request{}
	}
	//initialize the service by creating instance from the required service
	logging.Logger.Debug("initialize the service by creating instance from the required service")
	requiredService := service.GetService(i.vendor, i.serviceName, i.methodName,
		&http_message.HttpMsg{Request: i.req.Request, Response: i.req.Response})

	logging.Logger.Debug("calling Processing func to process the http message which encapsulated inside the ICAP request")
	//calling Processing func to process the http message which encapsulated inside the ICAP request
	IcapStatusCode, httpMsg, serviceHeaders := requiredService.Processing(partial)

	// adding the headers which the service wants to add them in the ICAP response
	logging.Logger.Debug("adding the headers which the service wants to add them in the ICAP response")
	if serviceHeaders != nil {
		for key, value := range serviceHeaders {
			i.h[key] = []string{value}
		}
	}

	//checking if shadow service mode is enabled to add logs instead of returning another
	//ICAP response beside the one who was sent to the client in line 88
	logging.Logger.Debug("checking if shadow service mode is enabled to add logs instead of returning another")
	if i.isShadowServiceEnabled {
		//add logs here
		return
	}

	//check the ICAP status code which returned from the service to decide
	//how should be the ICAP response
	switch IcapStatusCode {
	case utils.InternalServerErrStatusCodeStr:
		logging.Logger.Debug(i.serviceName + " returned ICAP response with status code " + strconv.Itoa(utils.InternalServerErrStatusCodeStr))
		i.w.WriteHeader(IcapStatusCode, nil, false)
		break
	case utils.Continue:
		logging.Logger.Debug(i.serviceName + " returned ICAP response with status code " + strconv.Itoa(utils.Continue))
		//in case the service returned 100 continue
		//we will get the rest of the body from the client
		httpMsgBody := i.preview()
		i.methodName = i.req.Method
		if i.req.Method == utils.ICAPModeReq {
			i.req.Request.Body = io.NopCloser(bytes.NewBuffer(httpMsgBody.Bytes()))
		} else {
			i.req.Response.Body = io.NopCloser(bytes.NewBuffer(httpMsgBody.Bytes()))
		}
		i.RespAndReqMods(false)
		break
	case utils.RequestTimeOutStatusCodeStr:
		logging.Logger.Debug(i.serviceName + " returned ICAP response with status code " + strconv.Itoa(utils.RequestTimeOutStatusCodeStr))
		i.w.WriteHeader(IcapStatusCode, nil, false)
		break
	case utils.NoModificationStatusCodeStr:
		logging.Logger.Debug(i.serviceName + " returned ICAP response with status code " + strconv.Itoa(utils.NoModificationStatusCodeStr))
		if i.Is204Allowed {
			i.w.WriteHeader(utils.NoModificationStatusCodeStr, nil, false)
		} else {
			i.w.WriteHeader(utils.OkStatusCodeStr, httpMsg, true)
		}
		break
	case utils.OkStatusCodeStr:
		logging.Logger.Debug(i.serviceName + " returned ICAP response with status code " + strconv.Itoa(utils.OkStatusCodeStr))
		i.w.WriteHeader(utils.OkStatusCodeStr, httpMsg, true)
		break
	case utils.BadRequestStatusCodeStr:
		logging.Logger.Debug(i.serviceName + " returned ICAP response with status code " + strconv.Itoa(utils.BadRequestStatusCodeStr))
		i.w.WriteHeader(IcapStatusCode, httpMsg, true)
		break
	}
}

// adding headers to the logging
func (i *ICAPRequest) addHeadersToLogs() {
	logging.Logger.Debug("printing ICAP request headers in logs")
	for key, element := range i.req.Header {
		res := key + " : "
		innerRes := ""
		for i := 0; i < len(element); i++ {
			innerRes += element[i]
			if i != len(element)-1 {
				innerRes += ", "
			}
		}
		res += innerRes
		logging.Logger.Debug("An ICAP request header -> " + res)
		res = ""
	}
}

// isServiceExists is a func to make sure that service which required in ICAP
// request is existing in the config.go file
func (i *ICAPRequest) isServiceExists() bool {
	services := i.appCfg.Services
	logging.Logger.Debug("looping over services exist in config.toml file to checking if the service doesn't exist or exist")
	for r := 0; r < len(services); r++ {
		if i.serviceName == services[r] {
			return true
		}
	}
	return false

}

// getMethodName is a func to get the name of the method of the ICAP request
func (i *ICAPRequest) getMethodName() string {
	logging.Logger.Debug("getting the method name")
	if i.methodName == "REQMOD" {
		i.methodName = "req_mode"
	} else if i.methodName == "RESPMOD" {
		i.methodName = "resp_mode"
	}
	return i.methodName
}

// isMethodAllowed is a func to check if the method in the ICAP request is allowed in config.go file or not
func (i *ICAPRequest) isMethodAllowed() bool {
	logging.Logger.Debug("checking if the method in the ICAP request is allowed in config.go file or not")
	if i.methodName == "RESPMOD" {
		return i.appCfg.ServicesInstances[i.serviceName].RespMode
	} else if i.methodName == "REQMOD" {
		return i.appCfg.ServicesInstances[i.serviceName].ReqMode

	}
	if i.methodName == "OPTIONS" {
		return true
	}
	return false
}

// getVendorName is a func to get the vendor of the service which in the ICAP request
func (i *ICAPRequest) getVendorName() string {
	logging.Logger.Debug("getting the vendor of the service which in the ICAP request")
	return i.appCfg.ServicesInstances[i.serviceName].Vendor
}

// addingISTAGServiceHeaders is a func to add the important header to ICAP response
func (i *ICAPRequest) addingISTAGServiceHeaders(ISTgValue string) {
	i.h["ISTag"] = []string{ISTgValue}
	i.h["Service"] = []string{i.appCfg.ServicesInstances[i.serviceName].ServiceCaption}
}

// is204Allowed is a func to check if ICAP request has the header "204 : Allowed" or not
func (i *ICAPRequest) is204Allowed() bool {
	logging.Logger.Debug("checking if (Allow : 204) header exists in ICAP request")
	Is204Allowed := false
	if _, exist := i.req.Header["Allow"]; exist &&
		i.req.Header.Get("Allow") == strconv.Itoa(utils.NoModificationStatusCodeStr) {
		Is204Allowed = true
	}
	return Is204Allowed
}

// shadowService is a func to apply the shadow service
func (i *ICAPRequest) shadowService() {
	logging.Logger.Debug("applying shadow service")
	if i.appCfg.DebuggingHeaders {
		logging.Logger.Debug("adding (X-ICAPeg-Shadow-Service : true) to ICAP response because this" +
			" configuration is enabled in config.toml file")
		i.h["X-ICAPeg-Shadow-Service"] = []string{"true"}
	}
	if i.Is204Allowed { // following RFC3507, if the request has Allow: 204 header, it is to be checked and if it doesn't exists, return the request as it is to the ICAP client, https://tools.ietf.org/html/rfc3507#section-4.6
		i.w.WriteHeader(utils.NoModificationStatusCodeStr, nil, false)
	} else {
		if i.req.Method == "REQMOD" {
			i.w.WriteHeader(utils.OkStatusCodeStr, i.req.Request, true)
			tempBody, _ := ioutil.ReadAll(i.req.Request.Body)
			i.w.Write(tempBody)
			i.req.Request.Body = io.NopCloser(bytes.NewBuffer(tempBody))
		} else if i.req.Method == "RESPMOD" {
			i.w.WriteHeader(utils.OkStatusCodeStr, i.req.Response, true)
			tempBody, _ := ioutil.ReadAll(i.req.Response.Body)
			i.w.Write(tempBody)
			i.req.Response.Body = io.NopCloser(bytes.NewBuffer(tempBody))
		}
	}
}

// getEnabledMethods is a func get all enable method of a specific service
func (i *ICAPRequest) getEnabledMethods() string {
	logging.Logger.Debug("getting all enable method of a specific service")
	var allMethods []string
	if i.appCfg.ServicesInstances[i.serviceName].RespMode {
		allMethods = append(allMethods, "RESPMOD")
	}
	if i.appCfg.ServicesInstances[i.serviceName].ReqMode {
		allMethods = append(allMethods, "REQMOD")
	}
	if len(allMethods) == 1 {
		return allMethods[0]
	}
	return allMethods[0] + ", " + allMethods[1]
}

func (i *ICAPRequest) servicePreview() (bool, string) {
	return i.appCfg.ServicesInstances[i.serviceName].PreviewEnabled,
		i.appCfg.ServicesInstances[i.serviceName].PreviewBytes
}

// optionsMode is a func to return an ICAP response in OPTIONS mode
func (i *ICAPRequest) optionsMode(serviceName string) {
	logging.Logger.Debug("preparing headers in OPTIONS mode response")
	i.h.Set("Methods", i.getEnabledMethods())
	i.h.Set("Allow", "204")
	// Add preview if preview_enabled is true in config.go
	previewEnabled, previewBytes := i.servicePreview()
	if previewEnabled == true {
		if pb, _ := strconv.Atoi(previewBytes); pb >= 0 {
			i.h.Set("Preview", previewBytes)
		}
	}
	i.h.Set("Transfer-Preview", utils.Any)
	i.w.WriteHeader(http.StatusOK, nil, false)
}

// preview function is used to get the rest of the http message from the client after sending
// a preview about the body first
func (i *ICAPRequest) preview() *bytes.Buffer {
	logging.Logger.Debug("getting the rest of the body from client after the service returned ICAP " +
		"response with status code" + strconv.Itoa(utils.Continue))
	r := icap.GetTheRest()
	c := io.NopCloser(r)
	buf := new(bytes.Buffer)
	buf.ReadFrom(c)
	return buf
}
