package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const configTemplate = `# deployscope.yaml — cluster configuration
cluster:
  name: "my-cluster"
  environment: "production"
  region: "us-east-1"

# Cache TTL for K8s API queries (default: 30s)
# cache_ttl: 30s

# Label selector for workloads (default: requires app.kubernetes.io/version)
# label_selector: "app.kubernetes.io/version"
`

const annotationTemplate = `# Example deployscope.dev annotations for your Helm chart values.yaml
#
# Add these to your deployment/statefulset/daemonset metadata.annotations:
#
# deployscope:
#   owner: "team-platform"
#   tier: "critical"                      # critical, standard, best-effort
#   gitopsRepo: "github.com/org/infra"
#   gitopsPath: "clusters/prod/auth/"
#   oncall: "#platform-oncall"
#   runbook: "https://wiki.internal/auth-runbook"
#   dashboard: "https://grafana.internal/d/auth"
#   dependsOn: "postgres-platform,redis-shared"
#   healthEndpoint: "http://localhost:8080/health"
#
# In your Helm chart template:
#
#   annotations:
#     deployscope.dev/owner: {{ .Values.deployscope.owner | quote }}
#     deployscope.dev/tier: {{ .Values.deployscope.tier | quote }}
#     deployscope.dev/gitops-repo: {{ .Values.deployscope.gitopsRepo | quote }}
#     deployscope.dev/gitops-path: {{ .Values.deployscope.gitopsPath | quote }}
#     deployscope.dev/oncall: {{ .Values.deployscope.oncall | quote }}
#     deployscope.dev/runbook: {{ .Values.deployscope.runbook | quote }}
#     deployscope.dev/dashboard: {{ .Values.deployscope.dashboard | quote }}
#     deployscope.dev/depends-on: {{ .Values.deployscope.dependsOn | quote }}
#     deployscope.dev/health-endpoint: {{ .Values.deployscope.healthEndpoint | quote }}
#
# To opt out a workload from deployscope:
#   annotations:
#     deployscope.dev/ignore: "true"
#
# Kustomize patch example:
#
# apiVersion: apps/v1
# kind: Deployment
# metadata:
#   name: my-service
#   annotations:
#     deployscope.dev/owner: "team-platform"
#     deployscope.dev/tier: "critical"
`

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate deployscope.yaml config and example annotations",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := "deployscope.yaml"
			if _, err := os.Stat(configFile); err == nil {
				return fmt.Errorf("%s already exists — remove it first to regenerate", configFile)
			}

			if err := os.WriteFile(configFile, []byte(configTemplate), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", configFile, err)
			}
			fmt.Printf("Created %s\n", configFile)

			annotationFile := "deployscope-annotations.example.yaml"
			if err := os.WriteFile(annotationFile, []byte(annotationTemplate), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", annotationFile, err)
			}
			fmt.Printf("Created %s\n", annotationFile)

			fmt.Println("\nNext steps:")
			fmt.Println("  1. Edit deployscope.yaml with your cluster identity")
			fmt.Println("  2. Add deployscope.dev/* annotations to your Helm chart values")
			fmt.Println("  3. Run 'deployscope doctor' to check annotation coverage")

			return nil
		},
	}
}
