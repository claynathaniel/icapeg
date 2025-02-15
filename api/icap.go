package api

import (
	"icapeg/icap"
	"icapeg/logging"
)

// ToICAPEGServe is the ICAsP Request Handler for all modes and services:
func ToICAPEGServe(w icap.ResponseWriter, req *icap.Request) {
	logging.Logger.Info("a request was sent to ICAPeg")
	//Creating new instance from struct IcapRequest yo handle upcoming ICAP requests
	ICAPRequest := NewICAPRequest(w, req)

	//calling RequestInitialization to retrieve the important information from the ICAP request
	//and initialize the ICAP response
	err := ICAPRequest.RequestInitialization()
	if err != nil {
		return
	}
	// after initialization, we call RequestProcessing func to process the ICAP request with a service
	ICAPRequest.RequestProcessing()
}
