package http

var CodeIntelligenceRoutes = []string{
	"POST /v1/workspaces/{workspace_id}/code/grep",
	"POST /v1/workspaces/{workspace_id}/code/read-file",
	"POST /v1/workspaces/{workspace_id}/code/symbols",
	"POST /v1/workspaces/{workspace_id}/code/references",
	"POST /v1/workspaces/{workspace_id}/code/ownership",
}
