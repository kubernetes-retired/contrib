// Package bigip interacts with F5 BIG-IP systems using the REST API.
package bigip

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
)

// BigIP is a container for our session state.
type BigIP struct {
	Host      string
	User      string
	Password  string
	Token     string // if set, will be used instead of User/Password
	Transport *http.Transport
}

// APIRequest builds our request before sending it to the server.
type APIRequest struct {
	Method      string
	URL         string
	Body        string
	ContentType string
}

// RequestError contains information about any error we get from a request.
type RequestError struct {
	Code       int      `json:"code,omitempty"`
	Message    string   `json:"message,omitempty"`
	ErrorStack []string `json:"errorStack,omitempty"`
}

// Error returns the error message.
func (r *RequestError) Error() error {
	if r.Message != "" {
		return errors.New(r.Message)
	}

	return nil
}

// NewSession sets up our connection to the BIG-IP system.
func NewSession(host, user, passwd string) *BigIP {
	var url string
	if !strings.HasPrefix(host, "http") {
		url = fmt.Sprintf("https://%s", host)
	} else {
		url = host
	}
	return &BigIP{
		Host:     url,
		User:     user,
		Password: passwd,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

// NewTokenSession sets up our connection to the BIG-IP system, and
// instructs the session to use token authentication instead of Basic
// Auth. This is required when using an external authentication
// provider, such as Radius or Active Directory. loginProviderName is
// probably "tmos" but your environment may vary.
func NewTokenSession(host, user, passwd, loginProviderName string) (b *BigIP, err error) {
	type authReq struct {
		Username          string `json:"username"`
		Password          string `json:"password"`
		LoginProviderName string `json:"loginProviderName"`
	}
	type authResp struct {
		Token struct {
			Token string
		}
	}

	auth := authReq{
		user,
		passwd,
		loginProviderName,
	}

	marshalJSON, err := json.Marshal(auth)
	if err != nil {
		return
	}

	req := &APIRequest{
		Method:      "post",
		URL:         "mgmt/shared/authn/login",
		Body:        string(marshalJSON),
		ContentType: "application/json",
	}

	b = NewSession(host, user, passwd)
	resp, err := b.APICall(req)
	if err != nil {
		return
	}

	if resp == nil {
		err = fmt.Errorf("unable to acquire authentication token")
		return
	}

	var aresp authResp
	err = json.Unmarshal(resp, &aresp)
	if err != nil {
		return
	}

	if aresp.Token.Token == "" {
		err = fmt.Errorf("unable to acquire authentication token")
		return
	}

	b.Token = aresp.Token.Token

	return
}

// APICall is used to query the BIG-IP web API.
func (b *BigIP) APICall(options *APIRequest) ([]byte, error) {
	var req *http.Request
	client := &http.Client{Transport: b.Transport}
	var format string
	if strings.Contains(options.URL, "mgmt/") {
		format = "%s/%s"
	} else {
		format = "%s/mgmt/tm/%s"
	}
	url := fmt.Sprintf(format, b.Host, options.URL)
	body := bytes.NewReader([]byte(options.Body))
	req, _ = http.NewRequest(strings.ToUpper(options.Method), url, body)
	if b.Token != "" {
		req.Header.Set("X-F5-Auth-Token", b.Token)
	} else {
		req.SetBasicAuth(b.User, b.Password)
	}

	//fmt.Println("REQ -- ", options.Method, " ", url," -- ",options.Body)

	if len(options.ContentType) > 0 {
		req.Header.Set("Content-Type", options.ContentType)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	data, _ := ioutil.ReadAll(res.Body)

	if res.StatusCode >= 400 {
		if res.Header["Content-Type"][0] == "application/json" {
			return data, b.checkError(data)
		}

		return data, errors.New(fmt.Sprintf("HTTP %d :: %s", res.StatusCode, string(data[:])))
	}

	return data, nil
}

func (b *BigIP) iControlPath(parts []string) string {
	var buffer bytes.Buffer
	for i, p := range parts {
		buffer.WriteString(strings.Replace(p, "/", "~", -1))
		if i < len(parts)-1 {
			buffer.WriteString("/")
		}
	}
	return buffer.String()
}

//Generic delete
func (b *BigIP) delete(path ...string) error {
	req := &APIRequest{
		Method: "delete",
		URL:    b.iControlPath(path),
	}

	_, callErr := b.APICall(req)
	return callErr
}

func (b *BigIP) post(body interface{}, path ...string) error {
	marshalJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req := &APIRequest{
		Method:      "post",
		URL:         b.iControlPath(path),
		Body:        string(marshalJSON),
		ContentType: "application/json",
	}

	_, callErr := b.APICall(req)
	return callErr
}

func (b *BigIP) put(body interface{}, path ...string) error {
	marshalJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req := &APIRequest{
		Method:      "put",
		URL:         b.iControlPath(path),
		Body:        string(marshalJSON),
		ContentType: "application/json",
	}

	_, callErr := b.APICall(req)
	return callErr
}

//Get a url and populate an entity. If the entity does not exist (404) then the
//passed entity will be untouched and false will be returned as the second parameter.
//You can use this to distinguish between a missing entity or an actual error.
func (b *BigIP) getForEntity(e interface{}, path ...string) (error, bool) {
	req := &APIRequest{
		Method:      "get",
		URL:         b.iControlPath(path),
		ContentType: "application/json",
	}

	resp, err := b.APICall(req)
	if err != nil {
		var reqError RequestError
		json.Unmarshal(resp, &reqError)
		if reqError.Code == 404 {
			return nil, false
		}
		return err, false
	}

	err = json.Unmarshal(resp, e)
	if err != nil {
		return err, false
	}

	return nil, true
}

// checkError handles any errors we get from our API requests. It returns either the
// message of the error, if any, or nil.
func (b *BigIP) checkError(resp []byte) error {
	if len(resp) == 0 {
		return nil
	}

	var reqError RequestError

	err := json.Unmarshal(resp, &reqError)
	if err != nil {
		return errors.New(fmt.Sprintf("%s\n%s", err.Error(), string(resp[:])))
	}

	err = reqError.Error()
	if err != nil {
		return err
	}

	return nil
}

// Helper to copy between transfer objects and model objects to hide the myriad of boolean representations
// in the iControlREST api. DTO fields can be tagged with bool:"yes|enabled|true" to set what true and false
// marshal to.
func marshal(to, from interface{}) error {
	toVal := reflect.ValueOf(to).Elem()
	fromVal := reflect.ValueOf(from).Elem()
	toType := toVal.Type()
	for i := 0; i < toVal.NumField(); i++ {
		toField := toVal.Field(i)
		toFieldType := toType.Field(i)
		fromField := fromVal.FieldByName(toFieldType.Name)
		if fromField.Interface() != nil && fromField.Kind() == toField.Kind() {
			toField.Set(fromField)
		} else if toField.Kind() == reflect.Bool && fromField.Kind() == reflect.String {
			switch fromField.Interface() {
			case "yes", "enabled", "true":
				toField.SetBool(true)
				break
			case "no", "disabled", "false", "":
				toField.SetBool(false)
				break
			default:
				return fmt.Errorf("Unknown boolean conversion for %s: %s", toFieldType.Name, fromField.Interface())
			}
		} else if fromField.Kind() == reflect.Bool && toField.Kind() == reflect.String {
			tag := toFieldType.Tag.Get("bool")
			switch tag {
			case "yes":
				toField.SetString(toBoolString(fromField.Interface().(bool), "yes", "no"))
				break
			case "enabled":
				toField.SetString(toBoolString(fromField.Interface().(bool), "enabled", "disabled"))
				break
			case "true":
				toField.SetString(toBoolString(fromField.Interface().(bool), "true", "false"))
				break
			}
		} else {
			return fmt.Errorf("Unknown type conversion %s -> %s", fromField.Kind(), toField.Kind())
		}
	}
	return nil
}

func toBoolString(b bool, trueStr, falseStr string) string {
	if b {
		return trueStr
	}
	return falseStr
}
