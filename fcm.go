package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2/google"
)

const (
	// fcm_server_url fcm server url
	fcm_server_url = "https://fcm.googleapis.com/fcm/send"
	// fcm_v1_server_url fcm v1 server url
	fcm_v1_server_url = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
	// MAX_TTL the default ttl for a notification
	MAX_TTL = 2419200
	// Priority_HIGH notification priority
	Priority_HIGH = "high"
	// Priority_NORMAL notification priority
	Priority_NORMAL = "normal"
	// retry_after_header header name
	retry_after_header = "Retry-After"
	// error_key readable error caching !
	error_key = "error"
)

var (
	// retreyableErrors whether the error is a retryable
	retreyableErrors = map[string]bool{
		"Unavailable":         true,
		"InternalServerError": true,
	}

	// fcmServerUrl for testing purposes
	fcmServerUrl = fcm_server_url
)

// FcmClient stores the key and the Message (FcmMsg)
type FcmClient struct {
	ApiKey           string
	Message          FcmMsg
	UseV1Api         bool             // Flag to switch between legacy and v1 API
	V1ProjectID      string           // Project ID required for v1 API
	gcmCredentialsV2 GcmCredentialsV2 // Add this to store the credentials
}

// GcmCredentialsV2 represents the structure for credentials (assuming this is defined)
type GcmCredentialsV2 struct {
	Type                    string `json:"type" bson:"type"`
	ProjectID               string `json:"project_id" bson:"project_id"`
	PrivateKeyID            string `json:"private_key_id" bson:"private_key_id"`
	PrivateKey              string `json:"private_key" bson:"private_key"`
	ClientEmail             string `json:"client_email" bson:"client_email"`
	ClientID                string `json:"client_id" bson:"client_id"`
	AuthURI                 string `json:"auth_uri" bson:"auth_uri"`
	TokenURI                string `json:"token_uri" bson:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url" bson:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url" bson:"client_x509_cert_url"`
}

type FcmV1ErrorDetail struct {
	Type      string `json:"@type"`
	ErrorCode string `json:"errorCode"`
}

type FcmV1Error struct {
	Code    int                `json:"code"`
	Message string             `json:"message"`
	Status  string             `json:"status"`
	Details []FcmV1ErrorDetail `json:"details"`
}

type FcmV1Response struct {
	Name  string     `json:"name"`
	Error FcmV1Error `json:"error"`
}

// gcmCredentialsV2ToJSON converts GcmCredentialsV2 to JSON bytes (assuming this method is defined)
func (c *GcmCredentialsV2) gcmCredentialsV2ToJSON() ([]byte, error) {
	// Implement the conversion to JSON
	return json.Marshal(c)
}

// getToken retrieves the OAuth2 token for FCM v1 API
func (f *FcmClient) getToken() (string, error) {
	credentialsJSON, err := f.gcmCredentialsV2.gcmCredentialsV2ToJSON()
	if err != nil {
		log.Printf("Error converting credentials to JSON: %v", err)
		return "", err
	}

	// fmt.Printf("credentialsJSON: %+v\n", string(credentialsJSON))

	// log.Println("Credentials successfully converted to JSON")

	config, err := google.JWTConfigFromJSON(credentialsJSON, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		log.Printf("Error creating JWT config from JSON: %v", err)
		return "", err
	}

	// log.Println("JWT config successfully created from JSON")

	tokenSource := config.TokenSource(context.Background())
	token, err := tokenSource.Token()
	if err != nil {
		log.Printf("Error obtaining token: %v", err)
		return "", err
	}

	// log.Printf("Token successfully obtained: %s", token.AccessToken)
	return token.AccessToken, nil
}

// FcmMsg represents fcm request message
type FcmMsg struct {
	Data                  interface{}         `json:"data,omitempty"`
	To                    string              `json:"to,omitempty"`
	RegistrationIds       []string            `json:"registration_ids,omitempty"`
	CollapseKey           string              `json:"collapse_key,omitempty"`
	Priority              string              `json:"priority,omitempty"`
	Notification          NotificationPayload `json:"notification,omitempty"`
	ContentAvailable      bool                `json:"content_available,omitempty"`
	DelayWhileIdle        bool                `json:"delay_while_idle,omitempty"`
	TimeToLive            int                 `json:"time_to_live,omitempty"`
	RestrictedPackageName string              `json:"restricted_package_name,omitempty"`
	DryRun                bool                `json:"dry_run,omitempty"`
	Condition             string              `json:"condition,omitempty"`
}

// FcmMsgV1 represents the v1 FCM request message
type FcmMsgV1 struct {
	Message V1Message `json:"message"`
}

type V1Message struct {
	Token        string              `json:"token,omitempty"`
	Topic        string              `json:"topic,omitempty"`
	Condition    string              `json:"condition,omitempty"`
	Notification NotificationPayload `json:"notification,omitempty"`
	Data         map[string]string   `json:"data,omitempty"`
	Android      *AndroidConfig      `json:"android,omitempty"`
	FcmOptions   *FcmOptions         `json:"fcm_options,omitempty"`
}

// AndroidConfig represents the configuration options for Android notifications
type AndroidConfig struct {
	CollapseKey           string                     `json:"collapse_key,omitempty"`
	Priority              string                     `json:"priority,omitempty"`
	Ttl                   string                     `json:"ttl,omitempty"`
	RestrictedPackageName string                     `json:"restricted_package_name,omitempty"`
	Data                  map[string]string          `json:"data,omitempty"`
	Notification          AndroidNotificationPayload `json:"notification,omitempty"`
}

// AndroidNotificationPayload represents the notification payload specific to Android
type AndroidNotificationPayload struct {
	Title             string `json:"title,omitempty"`
	Body              string `json:"body,omitempty"`
	Icon              string `json:"icon,omitempty"`
	Color             string `json:"color,omitempty"`
	Sound             string `json:"sound,omitempty"`
	NotificationCount int    `json:"notification_count,omitempty"`
	Tag               string `json:"tag,omitempty"`
	ClickAction       string `json:"click_action,omitempty"`
	BodyLocKey        string `json:"body_loc_key,omitempty"`
	BodyLocArgs       string `json:"body_loc_args,omitempty"`
	TitleLocKey       string `json:"title_loc_key,omitempty"`
	TitleLocArgs      string `json:"title_loc_args,omitempty"`
	ChannelID         string `json:"channel_id,omitempty"`
}

// FcmOptions represents the options for FCM messages
type FcmOptions struct {
	AnalyticsLabel string `json:"analytics_label,omitempty"`
}

// Define additional structures for AndroidConfig, WebpushConfig, ApnsConfig, and FcmOptions if needed

// FcmResponseStatus represents fcm response message - (tokens and topics)
type FcmResponseStatus struct {
	Ok            bool
	StatusCode    int
	MulticastId   int64               `json:"multicast_id"`
	Success       int                 `json:"success"`
	Fail          int                 `json:"failure"`
	Canonical_ids int                 `json:"canonical_ids"`
	Results       []map[string]string `json:"results,omitempty"`
	MsgId         int64               `json:"message_id,omitempty"`
	Err           string              `json:"error,omitempty"`
	RetryAfter    string
}

// NotificationPayload notification message payload
type NotificationPayload struct {
	Title            string `json:"title,omitempty"`
	Body             string `json:"body,omitempty"`
	Icon             string `json:"icon,omitempty"`
	Sound            string `json:"sound,omitempty"`
	Badge            string `json:"badge,omitempty"`
	Tag              string `json:"tag,omitempty"`
	Color            string `json:"color,omitempty"`
	ClickAction      string `json:"click_action,omitempty"`
	BodyLocKey       string `json:"body_loc_key,omitempty"`
	BodyLocArgs      string `json:"body_loc_args,omitempty"`
	TitleLocKey      string `json:"title_loc_key,omitempty"`
	TitleLocArgs     string `json:"title_loc_args,omitempty"`
	AndroidChannelID string `json:"android_channel_id,omitempty"`
}

// NewFcmClient init and create fcm client
// func NewFcmClient(apiKey string, useV1Api bool, v1ProjectID string, gcmCredentials GcmCredentialsV2) *FcmClient {
func NewFcmClient(apiKey string, gcmCredentials GcmCredentialsV2) *FcmClient {
	fcmc := new(FcmClient)
	fcmc.ApiKey = apiKey
	// check if gcmCredentials are provided and if provided, set the v1 API flag to true
	if gcmCredentials.ProjectID != "" {
		fcmc.UseV1Api = true
		fcmc.V1ProjectID = gcmCredentials.ProjectID
	} else {
		fcmc.UseV1Api = false
	}

	fcmc.gcmCredentialsV2 = gcmCredentials

	return fcmc
}

// NewFcmTopicMsg sets the targeted token/topic and the data payload
func (this *FcmClient) NewFcmTopicMsg(to string, body map[string]string) *FcmClient {
	this.NewFcmMsgTo(to, body)
	return this
}

// NewFcmMsgTo sets the targeted token/topic and the data payload
func (this *FcmClient) NewFcmMsgTo(to string, body interface{}) *FcmClient {
	this.Message.To = to
	this.Message.Data = body
	return this
}

// SetMsgData sets data payload
func (this *FcmClient) SetMsgData(body interface{}) *FcmClient {
	this.Message.Data = body
	return this
}

// NewFcmRegIdsMsg gets a list of devices with data payload
func (this *FcmClient) NewFcmRegIdsMsg(list []string, body interface{}) *FcmClient {
	this.newDevicesList(list)
	this.Message.Data = body
	return this
}

// newDevicesList init the devices list
func (this *FcmClient) newDevicesList(list []string) *FcmClient {
	this.Message.RegistrationIds = make([]string, len(list))
	copy(this.Message.RegistrationIds, list)
	return this
}

// AppendDevices adds more devices/tokens to the Fcm request
func (this *FcmClient) AppendDevices(list []string) *FcmClient {
	this.Message.RegistrationIds = append(this.Message.RegistrationIds, list...)
	return this
}

// apiKeyHeader generates the value of the Authorization key
func (this *FcmClient) apiKeyHeader() string {
	return fmt.Sprintf("key=%v", this.ApiKey)
}

// sendOnce send a single request to fcm
func (this *FcmClient) sendOnce() (*FcmResponseStatus, error) {
	fcmRespStatus := new(FcmResponseStatus)

	var jsonByte []byte
	var err error
	var request *http.Request

	if this.UseV1Api {
		// test1234, err := this.Message.toJsonByte()
		// fmt.Println("@@@@@@@@@@@@jsonByte: ", string(test1234))
		if err != nil {
			fmt.Println("Error converting message to JSON:", err)
			return fcmRespStatus, err
		}

		v1Message := this.convertToV1Message()
		// fmt.Println("@@@@@@@@@@@@this.convertToV1Message: ", v1Message)
		jsonByte, err = json.Marshal(v1Message)
		// fmt.Println("@@@@@@@@@@@@jsonByte2(v1Message): ", string(jsonByte))
		if err != nil {
			return fcmRespStatus, err
		}

		token, err := this.getToken()
		if err != nil {
			return fcmRespStatus, err
		}

		request, err = http.NewRequest("POST", fmt.Sprintf(fcm_v1_server_url, this.V1ProjectID), bytes.NewBuffer(jsonByte))
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %v", token))
	} else {
		jsonByte, err = this.Message.toJsonByte()
		fmt.Println("@@@@@@@@@@@@jsonByte: ", string(jsonByte))
		if err != nil {
			return fcmRespStatus, err
		}
		request, err = http.NewRequest("POST", fcmServerUrl, bytes.NewBuffer(jsonByte))
		request.Header.Set("Authorization", this.apiKeyHeader())
	}
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)

	// Logging the response details
	// log.Printf("HTTP Status: %s", response.Status)
	// log.Printf("HTTP Headers: %v", response.Header)

	if err != nil {
		return fcmRespStatus, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fcmRespStatus, err
	}

	log.Printf("HTTP Body: %s", string(body))

	fcmRespStatus.StatusCode = response.StatusCode
	fcmRespStatus.RetryAfter = response.Header.Get(retry_after_header)

	if response.StatusCode != 200 {
		if this.UseV1Api {
			// fmt.Println("Debug - response.StatusCode == 400")
			err = fcmRespStatus.parseStatusBodyV1(body)
			// this is because the response in the legacy API is always 200
			fcmRespStatus.StatusCode = 200

			// fmt.Println("Debug - err:", err)
			// fmt.Println("Debug - fcmRespStatus:", fcmRespStatus)

			if err == nil {
				return fcmRespStatus, nil
			} else {
				return fcmRespStatus, err
			}
		}

		return fcmRespStatus, nil
	}

	if this.UseV1Api {
		err = fcmRespStatus.parseStatusBodyV1(body)
	} else {
		err = fcmRespStatus.parseStatusBody(body)
	}

	if err != nil {
		return fcmRespStatus, err
	}
	fcmRespStatus.Ok = true

	fmt.Println("Debug - fcmRespStatus:", fcmRespStatus)
	return fcmRespStatus, nil
}

// Send to fcm
func (this *FcmClient) Send() (*FcmResponseStatus, error) {
	return this.sendOnce()
}

// Function to flatten the JSON structure into a map[string]string
func flattenMap(prefix string, input map[string]interface{}, output map[string]string) {
	for key, value := range input {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		switch v := value.(type) {
		case string:
			output[fullKey] = v
		case float64: // JSON numbers are float64 by default
			output[fullKey] = fmt.Sprintf("%v", v)
		case []interface{}:
			for i, elem := range v {
				output[fmt.Sprintf("%s.%d", fullKey, i)] = fmt.Sprintf("%v", elem)
			}
		case map[string]interface{}:
			flattenMap(fullKey, v, output)
		default:
			output[fullKey] = fmt.Sprintf("%v", v)
		}
	}
}

func (this *FcmClient) convertToV1Message() map[string]interface{} {
	// Ensure data is of type map[string]interface{}
	data, ok := this.Message.Data.(map[string]interface{})
	if !ok {
		// fmt.Println("Debug - data is not a map[string]interface{}")
		data = make(map[string]interface{})
	}

	// Filter and process data to ensure compatibility with FCM
	v1Data := make(map[string]string)

	flattenMap("", data, v1Data)

	// Stringify the original data map
	stringifiedData, err := json.Marshal(data)
	if err != nil {
		stringifiedData = []byte("{}") // Fallback to empty JSON object if marshalling fails
	}

	// Inject the stringified original data into v1Data
	v1Data["data"] = string(stringifiedData)

	notification := this.Message.Notification

	// fmt.Println("Debug - notification:", notification)

	// Use alert if title is not provided
	title := safeString(notification.Title)
	body := safeString(notification.Body)

	// Check if "alert" is present in data and is a string
	if alert, exists := data["alert"].(string); exists && alert != "" {
		title = alert
	}

	// if notification_count is provided, set it to what is provided, else set it to 0

	notification_count := 0
	var notification_err error
	notification_badge := safeString(notification.Badge)
	if notification_badge != "" {
		notification_count, notification_err = strconv.Atoi(notification_badge)
		if notification_err != nil {
			notification_count = 0
		}
	}

	// Debug prints
	// fmt.Println("Debug - data:", data)
	// fmt.Println("Debug - filteredData:", filteredData)
	// fmt.Println("Debug - notification:", notification)

	v1Message := map[string]interface{}{
		"message": map[string]interface{}{
			"token": this.Message.To,
			"notification": map[string]interface{}{
				"title": title,
				"body":  body,
			},
			"data": v1Data,
			"android": map[string]interface{}{
				"collapse_key":            safeString(this.Message.CollapseKey),
				"priority":                safeString(this.Message.Priority),
				"ttl":                     fmt.Sprintf("%ds", this.Message.TimeToLive),
				"restricted_package_name": safeString(this.Message.RestrictedPackageName),
				"data":                    v1Data,
				"notification": map[string]interface{}{
					"title":              title,
					"body":               body,
					"icon":               safeString(notification.Icon),
					"color":              safeString(notification.Color),
					"sound":              safeString(notification.Sound),
					"notification_count": notification_count,
					"tag":                safeString(notification.Tag),
					"click_action":       safeString(notification.ClickAction),
					"body_loc_key":       safeString(notification.BodyLocKey),
					"body_loc_args":      safeString(notification.BodyLocArgs),
					"title_loc_key":      safeString(notification.TitleLocKey),
					"title_loc_args":     safeString(notification.TitleLocArgs),
					"channel_id":         safeString(notification.AndroidChannelID),
					// "notification_count": 1,
				},
			},
		},
	}

	// fmt.Println("Debug - v1Message:", v1Message)

	return v1Message
}

// safeString is a helper function that ensures the value is a string, or returns an empty string if nil or not a string.
func safeString(value interface{}) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

// toJsonByte converts FcmMsg to a json byte
func (this *FcmMsg) toJsonByte() ([]byte, error) {
	return json.Marshal(this)
}

func (this *FcmResponseStatus) parseStatusBody(body []byte) error {
	if err := json.Unmarshal([]byte(body), &this); err != nil {
		return err
	}
	return nil
}

func (this *FcmResponseStatus) parseStatusBodyV1(body []byte) error {

	// Try parsing as FCM v1 API
	var v1Resp FcmV1Response
	if err := json.Unmarshal(body, &v1Resp); err != nil {
		// fmt.Println("Error parsing response body1:", err)
		return err
	}

	// Normalize v1 response to legacy format
	this.MulticastId = 0   // No equivalent in v1, setting it to 0
	this.Success = 0       // Default to 0, will be updated based on error presence
	this.Fail = 0          // Default to 0, will be updated based on error presence
	this.Canonical_ids = 0 // No equivalent in v1
	this.Results = nil     // No equivalent in v1

	// Check if there was an error
	if v1Resp.Error.Message != "" {
		this.Fail = 1
		this.Results = append(this.Results, map[string]string{
			// "error": fmt.Sprintf("%s - %s", v1Resp.Error.Status, v1Resp.Error.Message),
			"error": v1Resp.Error.Status,
		})
		this.Err = v1Resp.Error.Message
		this.StatusCode = v1Resp.Error.Code
		this.Ok = false
	} else {
		this.Success = 1
		this.Results = append(this.Results, map[string]string{
			"message_id": v1Resp.Name,
		})
		this.MsgId = 1 // Setting a default value for MsgId since it's not available in v1
		this.Ok = true
		this.StatusCode = 200
	}

	return nil
}

// SetPriority Sets the priority of the message.
// Priority_HIGH or Priority_NORMAL
func (this *FcmClient) SetPriority(p string) *FcmClient {
	if p == Priority_HIGH {
		this.Message.Priority = Priority_HIGH
	} else {
		this.Message.Priority = Priority_NORMAL
	}
	return this
}

// SetCollapseKey This parameter identifies a group of messages
// (e.g., with collapse_key: "Updates Available") that can be collapsed,
// so that only the last message gets sent when delivery can be resumed.
// This is intended to avoid sending too many of the same messages when the
// device comes back online or becomes active (see delay_while_idle).
func (this *FcmClient) SetCollapseKey(val string) *FcmClient {
	this.Message.CollapseKey = val
	return this
}

// SetNotificationPayload sets the notification payload based on the specs
// https://firebase.google.com/docs/cloud-messaging/http-server-ref
func (this *FcmClient) SetNotificationPayload(payload *NotificationPayload) *FcmClient {
	this.Message.Notification = *payload
	return this
}

// SetContentAvailable On iOS, use this field to represent content-available
// in the APNS payload. When a notification or message is sent and this is set
// to true, an inactive client app is awoken. On Android, data messages wake
// the app by default. On Chrome, currently not supported.
func (this *FcmClient) SetContentAvailable(isContentAvailable bool) *FcmClient {
	this.Message.ContentAvailable = isContentAvailable
	return this
}

// SetDelayWhileIdle When this parameter is set to true, it indicates that
// the message should not be sent until the device becomes active.
// The default value is false.
func (this *FcmClient) SetDelayWhileIdle(isDelayWhileIdle bool) *FcmClient {
	this.Message.DelayWhileIdle = isDelayWhileIdle
	return this
}

// SetTimeToLive This parameter specifies how long (in seconds) the message
// should be kept in FCM storage if the device is offline. The maximum time
// to live supported is 4 weeks, and the default value is 4 weeks.
// For more information, see
// https://firebase.google.com/docs/cloud-messaging/concept-options#ttl
func (this *FcmClient) SetTimeToLive(ttl int) *FcmClient {
	if ttl > MAX_TTL {
		this.Message.TimeToLive = MAX_TTL
	} else {
		this.Message.TimeToLive = ttl
	}
	return this
}

// SetRestrictedPackageName This parameter specifies the package name of the
// application where the registration tokens must match in order to
// receive the message.
func (this *FcmClient) SetRestrictedPackageName(pkg string) *FcmClient {
	this.Message.RestrictedPackageName = pkg
	return this
}

// SetDryRun This parameter, when set to true, allows developers to test
// a request without actually sending a message.
// The default value is false
func (this *FcmClient) SetDryRun(drun bool) *FcmClient {
	this.Message.DryRun = drun
	return this
}

// PrintResults prints the FcmResponseStatus results for fast using and debugging
func (this *FcmResponseStatus) PrintResults() {
	fmt.Println("Status Code   :", this.StatusCode)
	fmt.Println("Success       :", this.Success)
	fmt.Println("Fail          :", this.Fail)
	fmt.Println("Canonical_ids :", this.Canonical_ids)
	fmt.Println("Topic MsgId   :", this.MsgId)
	fmt.Println("Topic Err     :", this.Err)
	for i, val := range this.Results {
		fmt.Printf("Result(%d)> \n", i)
		for k, v := range val {
			fmt.Println("\t", k, " : ", v)
		}
	}
}

// IsTimeout check whether the response timeout based on http response status
// code and if any error is retryable
func (this *FcmResponseStatus) IsTimeout() bool {
	if this.StatusCode >= 500 {
		return true
	} else if this.StatusCode == 200 {
		for _, val := range this.Results {
			for k, v := range val {
				if k == error_key && retreyableErrors[v] == true {
					return true
				}
			}
		}
	}
	return false
}

// GetRetryAfterTime  fs the retrey after response header
// to a time.Duration
func (this *FcmResponseStatus) GetRetryAfterTime() (t time.Duration, e error) {
	t, e = time.ParseDuration(this.RetryAfter)
	return
}

// SetCondition to set a logical expression of conditions that determine the message target
func (this *FcmClient) SetCondition(condition string) *FcmClient {
	this.Message.Condition = condition
	return this
}
