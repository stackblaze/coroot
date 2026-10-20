package model

import (
	"regexp"
	"strings"

	"github.com/coroot/coroot/timeseries"
)

type ContainerStatus string

const (
	ContainerStatusWaiting    ContainerStatus = "waiting"
	ContainerStatusRunning    ContainerStatus = "running"
	ContainerStatusTerminated ContainerStatus = "terminated"
)

type DNSRequest struct {
	Type   string
	Domain string
}

type Container struct {
	Id   string
	Name string

	InitContainer bool

	ApplicationTypes map[ApplicationType]bool

	Image string

	Status   ContainerStatus
	Reason   string
	Ready    bool
	Restarts *timeseries.TimeSeries

	LastTerminatedReason string

	CpuLimit      *timeseries.TimeSeries
	CpuRequest    *timeseries.TimeSeries
	CpuUsage      *timeseries.TimeSeries
	CpuDelay      *timeseries.TimeSeries
	ThrottledTime *timeseries.TimeSeries

	MemoryRss *timeseries.TimeSeries

	MemoryCache        *timeseries.TimeSeries
	MemoryLimit        *timeseries.TimeSeries
	MemoryRequest      *timeseries.TimeSeries
	MemoryPressureSome *timeseries.TimeSeries
	MemoryPressureFull *timeseries.TimeSeries

	OOMKills *timeseries.TimeSeries
}

func NewContainer(id, name string) *Container {
	return &Container{
		Id:               id,
		Name:             name,
		ApplicationTypes: map[ApplicationType]bool{},
	}
}

var (
	deploymentPodRegex  = regexp.MustCompile(`(/k8s/[a-z0-9-]+/[a-z0-9-]+)-[0-9a-f]{1,10}-[bcdfghjklmnpqrstvwxz2456789]{5}/.+`)
	daemonsetPodRegex   = regexp.MustCompile(`(/k8s/[a-z0-9-]+/[a-z0-9-]+)-[bcdfghjklmnpqrstvwxz2456789]{5}/.+`)
	statefulsetPodRegex = regexp.MustCompile(`(/k8s/[a-z0-9-]+/[a-z0-9-]+)-\d+/.+`)
)

func ContainerIdToServiceName(containerId string) string {
	if !strings.HasPrefix(containerId, "/k8s/") {
		return containerId
	}
	for _, r := range []*regexp.Regexp{deploymentPodRegex, daemonsetPodRegex, statefulsetPodRegex} {
		if g := r.FindStringSubmatch(containerId); len(g) == 2 {
			return g[1]
		}
	}
	return containerId
}

func guessService(services []string, appId ApplicationId) string {
	for _, s := range services {
		if s == appId.Name {
			return s
		}
	}
	for _, s := range services {
		parts := strings.Split(s, "/")
		switch {
		case len(parts) == 4 && parts[1] == "k8s" && parts[2] == appId.Namespace && parts[3] == appId.Name:
		case len(parts) == 4 && parts[1] == "k8s-cronjob" && parts[2] == appId.Namespace && parts[3] == appId.Name:
		case len(parts) == 3 && parts[1] == "system.slice" && parts[2] == appId.Name+".service":
		case strings.HasSuffix(s, appId.Name): // /docker/backend <-> backend
		case strings.HasSuffix(appId.Name, s): // demo-cartservice <-> cartservice
		default:
			continue
		}
		return s
	}
	return ""
}

// guessServiceExact is a strict variant of guessService: a service matches
// only when it equals the application name, or is the application's exact
// /k8s/<namespace>/<name> (or /k8s-cronjob/...) path. Suffix matches are not
// considered.
func guessServiceExact(services []string, appId ApplicationId) string {
	for _, s := range services {
		if s == appId.Name {
			return s
		}
	}
	for _, s := range services {
		parts := strings.Split(s, "/")
		if len(parts) == 4 && (parts[1] == "k8s" || parts[1] == "k8s-cronjob") && parts[2] == appId.Namespace && parts[3] == appId.Name {
			return s
		}
	}
	return ""
}

// GuessServiceExact resolves the application's service using exact matching
// only, and only when no other application in w resolves to the same service.
// It is used where the result scopes which telemetry a user may read.
func GuessServiceExact(services []string, w *World, app *Application) string {
	service := guessServiceExact(services, app.Id)
	if service == "" {
		return ""
	}
	if w == nil {
		return service
	}
	for id := range w.Applications {
		if app.Id == id {
			continue
		}
		if guessServiceExact(services, id) == service || guessService(services, id) == service {
			return ""
		}
	}
	return service
}

func GuessService(services []string, w *World, app *Application) string {
	service := guessService(services, app.Id)
	if service == "" {
		return ""
	}
	for id := range w.Applications {
		if app.Id == id {
			continue
		}
		if s := guessService(services, id); s == service {
			return ""
		}
	}
	return service
}
