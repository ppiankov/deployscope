package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ppiankov/deployscope/internal/k8s"
)

// StatusOutput is the structured JSON output for status command.
type StatusOutput struct {
	Summary  k8s.Summary         `json:"summary"`
	Routing  *RoutingAdvice      `json:"routing,omitempty"`
	Services []k8s.ServiceStatus `json:"services"`
}

// RoutingAdvice provides deterministic next-step guidance.
type RoutingAdvice struct {
	Action     string   `json:"action"`
	Reason     string   `json:"reason"`
	Targets    []string `json:"targets,omitempty"`
	WOPriority string   `json:"suggested_wo_priority"`
}

func newStatusCmd() *cobra.Command {
	var format string
	var unhealthy bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show workload health status (one-shot, exits)",
		RunE: func(cmd *cobra.Command, args []string) error {
			k8sClient, err := k8s.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			services, summary, err := k8sClient.FetchDeployments(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to fetch workloads: %w", err)
			}

			if unhealthy {
				var filtered []k8s.ServiceStatus
				for _, svc := range services {
					if svc.Status != "green" {
						filtered = append(filtered, svc)
					}
				}
				services = filtered
			}

			routing := computeRouting(services, summary)

			if format == "json" {
				output := StatusOutput{
					Summary:  summary,
					Routing:  routing,
					Services: services,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			printStatusTable(services, summary, routing)

			// Exit code based on health
			if summary.Down > 0 || summary.Degraded > 0 {
				os.Exit(2)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json")
	cmd.Flags().BoolVar(&unhealthy, "unhealthy", false, "Show only degraded/down workloads")

	return cmd
}

func computeRouting(services []k8s.ServiceStatus, summary k8s.Summary) *RoutingAdvice {
	if summary.Down == 0 && summary.Degraded == 0 {
		return &RoutingAdvice{
			Action:     "proceed",
			Reason:     "all services healthy",
			WOPriority: "",
		}
	}

	// Check for critical-tier down services
	var criticalDown, criticalDegraded []k8s.ServiceStatus
	var oncallTargets []string
	seen := map[string]bool{}

	for _, svc := range services {
		tier := "standard"
		if svc.Tier != nil {
			tier = *svc.Tier
		}

		if tier == "critical" && svc.Status == "red" {
			criticalDown = append(criticalDown, svc)
			if svc.Integration.Oncall != nil && !seen[*svc.Integration.Oncall] {
				oncallTargets = append(oncallTargets, *svc.Integration.Oncall)
				seen[*svc.Integration.Oncall] = true
			}
		} else if tier == "critical" && svc.Status == "yellow" {
			criticalDegraded = append(criticalDegraded, svc)
		}
	}

	if len(criticalDown) > 0 {
		return &RoutingAdvice{
			Action:     "escalate",
			Reason:     fmt.Sprintf("%d critical-tier service(s) down", len(criticalDown)),
			Targets:    oncallTargets,
			WOPriority: "P0",
		}
	}

	if len(criticalDegraded) > 0 {
		return &RoutingAdvice{
			Action:     "warn",
			Reason:     fmt.Sprintf("%d critical-tier service(s) degraded", len(criticalDegraded)),
			WOPriority: "P1",
		}
	}

	if summary.Down > 0 {
		return &RoutingAdvice{
			Action:     "inform",
			Reason:     fmt.Sprintf("%d service(s) down", summary.Down),
			WOPriority: "P1",
		}
	}

	return &RoutingAdvice{
		Action:     "log",
		Reason:     fmt.Sprintf("%d service(s) degraded", summary.Degraded),
		WOPriority: "P2",
	}
}

func printStatusTable(services []k8s.ServiceStatus, summary k8s.Summary, routing *RoutingAdvice) {
	fmt.Printf("Summary: %d total, %d healthy, %d degraded, %d down\n",
		summary.Total, summary.Healthy, summary.Degraded, summary.Down)

	if routing != nil && routing.Action != "proceed" {
		fmt.Printf("Routing: %s — %s", routing.Action, routing.Reason)
		if len(routing.Targets) > 0 {
			fmt.Printf(" → %v", routing.Targets)
		}
		fmt.Println()
	}

	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "STATUS\tNAME\tNAMESPACE\tTYPE\tREPLICAS\tVERSION\tOWNER\tTIER")
	for _, svc := range services {
		owner := "-"
		if svc.Owner != nil {
			owner = *svc.Owner
		}
		tier := "-"
		if svc.Tier != nil {
			tier = *svc.Tier
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d/%d\t%s\t%s\t%s\n",
			svc.Status, svc.Name, svc.Namespace, svc.WorkloadType,
			svc.ReadyReplicas, svc.Replicas, svc.Version, owner, tier)
	}
	_ = w.Flush()
}
