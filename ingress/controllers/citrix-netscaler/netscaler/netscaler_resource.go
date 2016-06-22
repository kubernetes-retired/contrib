package netscaler

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

func createResource(resourceType string, resourceJson []byte) ([]byte, error) {
	log.Println("Creating resource of type ", resourceType)
	nsIp := os.Getenv("NS_IP")
	username := os.Getenv("NS_USERNAME")
	password := os.Getenv("NS_PASSWORD")

	url := fmt.Sprintf("http://%s/nitro/v1/config/%s", nsIp, resourceType)

	method := "POST"
	if strings.HasSuffix(resourceType, "_binding") {
		method = "POST"
	}

	var contentType = fmt.Sprintf("application/vnd.com.citrix.netscaler.%s+json", resourceType)

	req, err := http.NewRequest(method, url, bytes.NewBuffer(resourceJson))
	req.Header.Set("Accept", contentType)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-NITRO-USER", username)
	req.Header.Set("X-NITRO-PASS", password)
	//req.Header.Set("X-NITRO-ONERROR:continue") TODO
	log.Println("url:", url)
	log.Println("resourceJson:", string(resourceJson))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
		return []byte{}, err
	} else {
		defer resp.Body.Close()
		log.Println("response Status:", resp.Status)

		switch resp.Status {
		case "201 Created", "200 OK", "409 Conflict":
			body, _ := ioutil.ReadAll(resp.Body)
			return body, nil

		case "207 Multi Status":
			//TODO
			body, _ := ioutil.ReadAll(resp.Body)
			return body, err
		case "400 Bad Request", "401 Unauthorized", "403 Forbidden",
			"404 Not Found", "405 Method Not Allowed", "406 Not Acceptable",
			"503 Service Unavailable", "599 Netscaler specific error":
			//TODO
			body, _ := ioutil.ReadAll(resp.Body)
			log.Println("error = " + string(body))
			return body, errors.New("failed: " + resp.Status + " (" + string(body) + ")")
		default:
			body, err := ioutil.ReadAll(resp.Body)
			return body, err

		}
	}
}

func deleteResource(resourceType string, resourceName string) ([]byte, error) {
	log.Println("Deleting resource of type ", resourceType)
	nsIp := os.Getenv("NS_IP")
	username := os.Getenv("NS_USERNAME")
	password := os.Getenv("NS_PASSWORD")
	url := fmt.Sprintf("http://%s/nitro/v1/config/%s", nsIp, resourceType)

	var contentType = fmt.Sprintf("application/vnd.com.citrix.netscaler.%s+json", resourceType)
	url = url + "/" + resourceName
	log.Println("url:", url)
	req, err := http.NewRequest("DELETE", url, bytes.NewBuffer([]byte{}))
	req.Header.Set("Accept", contentType)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-NITRO-USER", username)
	req.Header.Set("X-NITRO-PASS", password)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
		return []byte{}, err
	} else {
		defer resp.Body.Close()
		log.Println("response Status:", resp.Status)

		switch resp.Status {
		case "200 OK", "404 Not Found":
			body, _ := ioutil.ReadAll(resp.Body)
			return body, nil

		case "400 Bad Request", "401 Unauthorized", "403 Forbidden",
			"405 Method Not Allowed", "406 Not Acceptable",
			"409 Conflict", "503 Service Unavailable", "599 Netscaler specific error":
			//TODO
			body, _ := ioutil.ReadAll(resp.Body)
			log.Println("error = " + string(body))
			return body, errors.New("failed: " + resp.Status + " (" + string(body) + ")")
		default:
			body, err := ioutil.ReadAll(resp.Body)
			return body, err

		}
	}
}

func unbindResource(resourceType string, resourceName string, boundResourceType string, boundResource string) ([]byte, error) {
	log.Println("Unbinding resource of type ", resourceType)
	nsIp := os.Getenv("NS_IP")
	username := os.Getenv("NS_USERNAME")
	password := os.Getenv("NS_PASSWORD")
	url := fmt.Sprintf("http://%s/nitro/v1/config/%s", nsIp, resourceType)

	var contentType = fmt.Sprintf("application/vnd.com.citrix.netscaler.%s+json", resourceType)
	url = url + "/" + resourceName + "?args=" + boundResourceType + ":" + boundResource

	log.Println("url:", url)
	req, err := http.NewRequest("DELETE", url, bytes.NewBuffer([]byte{}))
	req.Header.Set("Accept", contentType)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-NITRO-USER", username)
	req.Header.Set("X-NITRO-PASS", password)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
		return []byte{}, err
	} else {
		defer resp.Body.Close()
		log.Println("response Status:", resp.Status)

		switch resp.Status {
		case "200 OK", "404 Not Found":
			body, _ := ioutil.ReadAll(resp.Body)
			return body, nil

		case "400 Bad Request", "401 Unauthorized", "403 Forbidden",
			"405 Method Not Allowed", "406 Not Acceptable",
			"409 Conflict", "503 Service Unavailable", "599 Netscaler specific error":
			//TODO
			body, _ := ioutil.ReadAll(resp.Body)
			log.Println("error = " + string(body))
			return body, errors.New("failed: " + resp.Status + " (" + string(body) + ")")
		default:
			body, err := ioutil.ReadAll(resp.Body)
			log.Println("error = " + string(body))
			return body, err

		}
	}
}

func listBoundResources(resourceName string, resourceType string, boundResourceType string, boundResourceFilterName string, boundResourceFilterValue string) ([]byte, error) {
	log.Println("listing resource of type ", resourceType)
	nsIp := os.Getenv("NS_IP")
	username := os.Getenv("NS_USERNAME")
	password := os.Getenv("NS_PASSWORD")
	var url string
	if boundResourceFilterName == "" {
		url = fmt.Sprintf("http://%s/nitro/v1/config/%s_%s_binding/%s", nsIp, resourceType, boundResourceType, resourceName)
	} else {
		url = fmt.Sprintf("http://%s/nitro/v1/config/%s_%s_binding/%s?filter=%s:%s", nsIp, resourceType, boundResourceType, resourceName, boundResourceFilterName, boundResourceFilterValue)
	}

	var contentType = fmt.Sprintf("application/vnd.com.citrix.netscaler.%s_%s_binding+json", resourceType, boundResourceType)

	log.Println("url:", url)
	req, err := http.NewRequest("GET", url, bytes.NewBuffer([]byte{}))
	//req.Header.Set("Accept", contentType)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-NITRO-USER", username)
	req.Header.Set("X-NITRO-PASS", password)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
		return []byte{}, err
	} else {
		defer resp.Body.Close()
		log.Println("response Status:", resp.Status)

		switch resp.Status {
		case "200 OK":
			body, _ := ioutil.ReadAll(resp.Body)
			return body, nil
		case "400 Bad Request", "401 Unauthorized", "403 Forbidden", "404 Not Found",
			"405 Method Not Allowed", "406 Not Acceptable",
			"409 Conflict", "503 Service Unavailable", "599 Netscaler specific error":
			//TODO
			body, _ := ioutil.ReadAll(resp.Body)
			log.Println("error = " + string(body))
			return body, errors.New("failed: " + resp.Status + " (" + string(body) + ")")
		default:
			body, err := ioutil.ReadAll(resp.Body)
			log.Println("error = " + string(body))
			return body, err

		}
	}
}

func listResource(resourceType string, resourceName string) ([]byte, error) {
	log.Println("listing resource of type ", resourceType)
	nsIp := os.Getenv("NS_IP")
	username := os.Getenv("NS_USERNAME")
	password := os.Getenv("NS_PASSWORD")
	url := fmt.Sprintf("http://%s/nitro/v1/config/%s", nsIp, resourceType)

	if resourceName != "" {
		url = fmt.Sprintf("http://%s/nitro/v1/config/%s/%s", nsIp, resourceType, resourceName)
	}

	var contentType = fmt.Sprintf("application/vnd.com.citrix.netscaler.%s+json", resourceType)

	log.Println("url:", url)
	req, err := http.NewRequest("GET", url, bytes.NewBuffer([]byte{}))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-NITRO-USER", username)
	req.Header.Set("X-NITRO-PASS", password)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
		return []byte{}, err
	} else {
		defer resp.Body.Close()
		log.Println("response Status:", resp.Status)

		switch resp.Status {
		case "200 OK":
			body, _ := ioutil.ReadAll(resp.Body)
			return body, nil
		case "400 Bad Request", "401 Unauthorized", "403 Forbidden", "404 Not Found",
			"405 Method Not Allowed", "406 Not Acceptable",
			"409 Conflict", "503 Service Unavailable", "599 Netscaler specific error":
			//TODO
			body, _ := ioutil.ReadAll(resp.Body)
			log.Println("error = " + string(body))
			return body, errors.New("failed: " + resp.Status + " (" + string(body) + ")")
		default:
			body, err := ioutil.ReadAll(resp.Body)
			log.Println("error = " + string(body))
			return body, err

		}
	}
}
