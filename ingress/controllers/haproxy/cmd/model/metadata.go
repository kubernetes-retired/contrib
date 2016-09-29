package model

// Injected through ldfags
var (
	version    string
	date       string
	gitVersion string
	appInfo    *AppInfo
)

// AppInfo holds infor about the application
type AppInfo struct {
	Version    string
	Date       string
	GitVersion string
}

// GetAppInfo returns application info struct
func GetAppInfo() *AppInfo {

	if appInfo == nil {
		appInfo = &AppInfo{
			Version:    version,
			Date:       date,
			GitVersion: gitVersion,
		}
	}
	return appInfo
}
