package ecs

//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
// Code generated by Alibaba Cloud SDK Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

import (
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/responses"
)

// DescribeRenewalPrice invokes the ecs.DescribeRenewalPrice API synchronously
func (client *Client) DescribeRenewalPrice(request *DescribeRenewalPriceRequest) (response *DescribeRenewalPriceResponse, err error) {
	response = CreateDescribeRenewalPriceResponse()
	err = client.DoAction(request, response)
	return
}

// DescribeRenewalPriceWithChan invokes the ecs.DescribeRenewalPrice API asynchronously
func (client *Client) DescribeRenewalPriceWithChan(request *DescribeRenewalPriceRequest) (<-chan *DescribeRenewalPriceResponse, <-chan error) {
	responseChan := make(chan *DescribeRenewalPriceResponse, 1)
	errChan := make(chan error, 1)
	err := client.AddAsyncTask(func() {
		defer close(responseChan)
		defer close(errChan)
		response, err := client.DescribeRenewalPrice(request)
		if err != nil {
			errChan <- err
		} else {
			responseChan <- response
		}
	})
	if err != nil {
		errChan <- err
		close(responseChan)
		close(errChan)
	}
	return responseChan, errChan
}

// DescribeRenewalPriceWithCallback invokes the ecs.DescribeRenewalPrice API asynchronously
func (client *Client) DescribeRenewalPriceWithCallback(request *DescribeRenewalPriceRequest, callback func(response *DescribeRenewalPriceResponse, err error)) <-chan int {
	result := make(chan int, 1)
	err := client.AddAsyncTask(func() {
		var response *DescribeRenewalPriceResponse
		var err error
		defer close(result)
		response, err = client.DescribeRenewalPrice(request)
		callback(response, err)
		result <- 1
	})
	if err != nil {
		defer close(result)
		callback(nil, err)
		result <- 0
	}
	return result
}

// DescribeRenewalPriceRequest is the request struct for api DescribeRenewalPrice
type DescribeRenewalPriceRequest struct {
	*requests.RpcRequest
	ResourceOwnerId      requests.Integer                     `position:"Query" name:"ResourceOwnerId"`
	PriceUnit            string                               `position:"Query" name:"PriceUnit"`
	ResourceId           string                               `position:"Query" name:"ResourceId"`
	Period               requests.Integer                     `position:"Query" name:"Period"`
	ResourceOwnerAccount string                               `position:"Query" name:"ResourceOwnerAccount"`
	OwnerAccount         string                               `position:"Query" name:"OwnerAccount"`
	ExpectedRenewDay     requests.Integer                     `position:"Query" name:"ExpectedRenewDay"`
	OwnerId              requests.Integer                     `position:"Query" name:"OwnerId"`
	ResourceType         string                               `position:"Query" name:"ResourceType"`
	PromotionOptions     DescribeRenewalPricePromotionOptions `position:"Query" name:"PromotionOptions"  type:"Struct"`
}

// DescribeRenewalPricePromotionOptions is a repeated param struct in DescribeRenewalPriceRequest
type DescribeRenewalPricePromotionOptions struct {
	CouponNo string `name:"CouponNo"`
}

// DescribeRenewalPriceResponse is the response struct for api DescribeRenewalPrice
type DescribeRenewalPriceResponse struct {
	*responses.BaseResponse
	RequestId string    `json:"RequestId" xml:"RequestId"`
	PriceInfo PriceInfo `json:"PriceInfo" xml:"PriceInfo"`
}

// CreateDescribeRenewalPriceRequest creates a request to invoke DescribeRenewalPrice API
func CreateDescribeRenewalPriceRequest() (request *DescribeRenewalPriceRequest) {
	request = &DescribeRenewalPriceRequest{
		RpcRequest: &requests.RpcRequest{},
	}
	request.InitWithApiInfo("Ecs", "2014-05-26", "DescribeRenewalPrice", "ecs", "openAPI")
	request.Method = requests.POST
	return
}

// CreateDescribeRenewalPriceResponse creates a response to parse from DescribeRenewalPrice response
func CreateDescribeRenewalPriceResponse() (response *DescribeRenewalPriceResponse) {
	response = &DescribeRenewalPriceResponse{
		BaseResponse: &responses.BaseResponse{},
	}
	return
}
