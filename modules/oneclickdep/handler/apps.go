package handler

import (
	// "fmt"
	// "log"
	"net/http"
	"opensource/chaos/common"
	"opensource/chaos/common/fasthttp"
	"opensource/chaos/domain/model/marathon"
	"opensource/chaos/modules/oneclickdep/entity"
	webUtils "opensource/chaos/modules/oneclickdep/utils"
	"strconv"
	"strings"
)

// 逻辑为：放入仓库的时候，即每个模块携带时间戳，每次前端构建时候传入比如zookeeper
// 则后端则从私库里捞出zookeeper所有模块，并按时间倒叙取出最新的zk模块镜像进行部署
// 则，在回滚时候，捞出倒数第二新的模块进行重新部署。部署时候更新labels即可。
func RollbackAppsHandler(pathParams map[string]string, data []byte) interface{} {
	var request entity.RollbackAppsBatchRequest
	webUtils.ParseOuterRequest(data, &request)

	requestBatch := make([]entity.DeployAppsRequest, len(request.Batch))
	for i, v := range request.Batch {
		// TODO 通过ID把Image信息拿到，暂时认为ID和Image是等价的
		_, image, tag := common.DockerClient.GetPreviousImageAndTag(v.Id, v.Version, "")
		var request entity.DeployAppsRequest
		request.Id = v.Id
		request.Image = image
		request.Version = tag
		requestBatch[i] = request
	}
	appsBatchRequest := entity.DeployAppsBatchRequest{}
	appsBatchRequest.Batch = requestBatch
	resData, code := createOrUpdateAppsService(appsBatchRequest)
	return webUtils.ProcessResponse(code, resData)
}

func CreateAppsHandler(pathParams map[string]string, data []byte) interface{} {
	var request entity.DeployAppsRequest
	webUtils.ParseOuterRequest(data, &request)
	deployInfo := webUtils.BuildAppsRequest(request)
	var resData map[string]interface{}
	resCode := fasthttp.JsonReqAndResHandler(common.Path.MarathonAppsUrl, deployInfo, &resData, "POST")
	return webUtils.ProcessResponse(resCode, resData)
}

func CreateOrUpdateAppsHandler(pathParams map[string]string, data []byte) interface{} {
	var request entity.DeployAppsBatchRequest
	webUtils.ParseOuterRequest(data, &request)
	resData, resCode := createOrUpdateAppsService(request)
	return webUtils.ProcessResponse(resCode, resData)
}

func GetInfoAppsHandler(pathParams map[string]string, data []byte) interface{} {
	var marathonApps marathon.MarathonAppsGlobalInfoResponse
	fasthttp.JsonReqAndResHandler(common.Path.MarathonAppsUrl, nil, &marathonApps, "GET")
	appsCnt := len(marathonApps.Apps)

	// should not code like this: appsGlobalInfos := [appsCnt]entity.AppsGlobalInfo{}
	appsGlobalInfos := make([]entity.AppsGlobalInfoResponse, appsCnt)

	for i, v := range marathonApps.Apps {
		var perApp entity.AppsGlobalInfoResponse
		if strings.LastIndex(v.Id, "/") == -1 {
			perApp.Id = v.Id
		} else {
			perApp.Id = v.Id[strings.LastIndex(v.Id, "/")+1:]
		}
		perApp.Cpus = strconv.FormatFloat(v.Cpus, 'f', 1, 64)
		perApp.CurrentInstances = strconv.Itoa(v.TasksRunning)
		if strings.LastIndex(v.Id, "/") <= 0 { // exclude like /zk or zk
			perApp.Group = "No Groups"
		} else {
			perApp.Group = v.Id[0:strings.LastIndex(v.Id, "/")]
		}
		perApp.Instances = strconv.Itoa(v.Instances)
		perApp.Mem = strconv.FormatFloat(v.Mem, 'f', 1, 64)
		perApp.Healthy = strconv.FormatFloat(100*float64(v.TasksRunning)/float64(v.Instances), 'f', 1, 64)
		perApp.FormatStatus(v.TasksStaged)
		appsGlobalInfos[i] = perApp
	}
	return webUtils.ProcessResponseFully(http.StatusOK, appsGlobalInfos, false)
}

// TODO 此处需要从marathon拿取基本数据，再结合docker进行联合查询
// 逻辑：先从marathon取出所有task的实际物理IP以及端口映射关系。然后找到对应的物理机，
// 通过docker inspect和其对应上，并取出实际机器IP。最后一起返回
// 目的1：能够通过服务app查找到其每个实例的虚拟ip端口以及宿主机的ip端口。以便能进行ssh登录
// 目的2：辅助进行服务监控。能够识别所有的虚拟ip。需要和consul结合起来看。
func GetSingleAppsHandler(pathParams map[string]string, data []byte) interface{} {
	return nil
}

func DeleteAppsHandler(pathParams map[string]string, data []byte) interface{} {
	appId := pathParams["appId"]
	var resData map[string]interface{}
	resCode := fasthttp.JsonReqAndResHandler(common.Path.MarathonAppsUrl+"/"+appId, nil, &resData, "DELETE")
	return webUtils.ProcessResponse(resCode, resData)
}

func createOrUpdateAppsService(request entity.DeployAppsBatchRequest) (interface{}, int) {
	finalRequest := make([]marathon.MarathonAppsRequest, len(request.Batch))
	for i, v := range request.Batch {
		deployInfo := webUtils.BuildAppsRequest(v)
		finalRequest[i] = *deployInfo
	}
	var response map[string]interface{}
	code := fasthttp.JsonReqAndResHandler(common.Path.MarathonAppsUrl, finalRequest, &response, "PUT")
	return response, code
}
