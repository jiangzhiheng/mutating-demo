package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
)

var certFile string
var keyFile string

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func main() {
	flag.StringVar(&certFile, "certFile", "", "the webhook server certFile")
	flag.StringVar(&keyFile, "keyFile", "", "the webhook server keyFile")
	flag.Parse()

	router := gin.Default()
	router.POST("/mutating-demo", mutatingDeployment)

	// 启动HTTPS服务器
	err := router.RunTLS(":8003", certFile, keyFile)
	if err != nil {
		log.Fatal("Failed to start HTTPS server: ", err)
	}
}

func mutatingDeployment(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("Failed to read request body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}

	//解析 AdmissionReview 对象，也可以用Golang原生的JSON序列化器（encoding/json包），有兴趣网上可以查一下两种编解码器的区别
	admissionReview := &admissionv1.AdmissionReview{}
	if _,_,err := codecs.UniversalDeserializer().Decode(body,nil, admissionReview);err != nil{
			log.Errorf("Failed to parse AdmissionReview: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse AdmissionReview"})
			return
	}

	// 获取 deployment 对象
	deployment := &appsv1.Deployment{}
	rawObject := admissionReview.Request.Object.Raw
	if _,_,err := codecs.UniversalDeserializer().Decode(rawObject,nil ,deployment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse Deployment"})
		return
	}
	// 任务 1：给所有 deployment 资源加上 env-type=test 的 annotation
	annotationPatch,err := setAnnotationPatch(deployment)
	if err != nil{
		log.Errorf("Failed to set Deployment annotations: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to set Deployment annotations"})
		return
	}

	// 任务 2：验证 deployment 的副本数，如果大于 3 则修改为 3, 也就是副本数最大不能超过 3。
	if *deployment.Spec.Replicas > 3 {
		// 修改副本数为 1
		deployment.Spec.Replicas = new(int32)
		*deployment.Spec.Replicas = 3

		deploymentPatch := setReplicasPatch(deployment)
		// 构建允许相应
		finalPatch, err := mergePatches(deploymentPatch,annotationPatch)
		if err != nil{
			log.Errorf("merge patch failed: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "merge patch failed"})
			return
		}

		admissionReview.Response = &admissionv1.AdmissionResponse{
			Allowed: true,
			Patch:   finalPatch,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
		}
	} else {
		// 构建允许响应
		admissionReview.Response = &admissionv1.AdmissionResponse{
			Allowed: true,
			Patch:   annotationPatch,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
		}
	}
	// 设置相应 uid 和 api 版本
	admissionReview.Response.UID = admissionReview.Request.UID

	// 构建 AdmissionReview 响应
	responseBody, err := json.Marshal(admissionReview)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal AdmissionReview response"})
		return
	}

	// 发送 AdmissionReview 响应
	c.Data(http.StatusOK, "application/json", responseBody)
}

func setReplicasPatch(deployment *appsv1.Deployment) []byte {
	patch := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/spec/replicas",
			"value": *deployment.Spec.Replicas,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.Error("Failed to marshal patch:", err)
	}
	return patchBytes
}

func setAnnotationPatch(deployment *appsv1.Deployment) ([]byte, error) {
	annotationKey := "env-type"
	annotationValue := "test"

	annotations := deployment.ObjectMeta.GetAnnotations()
	_, exists := annotations[annotationKey]

	var patch []map[string]interface{}
	if exists {
		// 如果annotation存在，则使用"replace"操作
		patch = []map[string]interface{}{
			{
				"op":    "replace",
				"path":  "/metadata/annotations/" + annotationKey,
				"value": annotationValue,
			},
		}
	} else {
		// 如果annotation不存在，则使用"add"操作
		patch = []map[string]interface{}{
			{
				"op":    "add",
				"path":  "/metadata/annotations",
				"value": map[string]string{annotationKey: annotationValue},
			},
		}
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch: %v", err)
	}

	return patchBytes, nil
}

func mergePatches(patches ...[]byte) ([]byte, error) {
	mergedPatch := make([]map[string]interface{}, 0)

	for _, patch := range patches {
		var patchObj []map[string]interface{}
		if err := json.Unmarshal(patch, &patchObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal patch: %v", err)
		}

		mergedPatch = append(mergedPatch, patchObj...)
	}

	mergedPatchBytes, err := json.Marshal(mergedPatch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged patch: %v", err)
	}

	return mergedPatchBytes, nil
}
