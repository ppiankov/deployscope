package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ppiankov/deployscope/internal/k8s"
)

type DoctorOutput struct {
	K8sConnectivity  string             `json:"k8s_connectivity"`
	TotalWorkloads   int                `json:"total_workloads"`
	IgnoredWorkloads int                `json:"ignored_workloads"`
	Coverage         AnnotationCoverage `json:"annotation_coverage"`
	AgentReadiness   float64            `json:"agent_readiness"`
	Warnings         []string           `json:"warnings,omitempty"`
}

type AnnotationCoverage struct {
	Owner      float64 `json:"owner"`
	Tier       float64 `json:"tier"`
	GitOpsRepo float64 `json:"gitops_repo"`
	Oncall     float64 `json:"oncall"`
	Runbook    float64 `json:"runbook"`
	DependsOn  float64 `json:"depends_on"`
}

func newDoctorCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check K8s connectivity, RBAC, and annotation coverage",
		RunE: func(cmd *cobra.Command, args []string) error {
			k8sClient, err := k8s.NewClient()
			if err != nil {
				output := DoctorOutput{
					K8sConnectivity: fmt.Sprintf("error: %v", err),
				}
				if format == "json" {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					_ = enc.Encode(output)
				} else {
					fmt.Printf("K8s connectivity: FAIL (%v)\n", err)
				}
				os.Exit(1)
				return nil
			}

			if err := k8sClient.CheckReady(cmd.Context()); err != nil {
				output := DoctorOutput{
					K8sConnectivity: fmt.Sprintf("error: %v", err),
				}
				if format == "json" {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					_ = enc.Encode(output)
				} else {
					fmt.Printf("K8s connectivity: FAIL (%v)\n", err)
				}
				os.Exit(1)
				return nil
			}

			services, _, err := k8sClient.FetchDeployments(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to fetch workloads: %w", err)
			}

			total := len(services)
			coverage := computeCoverage(services)
			readiness := (coverage.Owner + coverage.Tier + coverage.GitOpsRepo + coverage.Oncall) / 4.0

			var warnings []string
			if coverage.Owner < 0.5 {
				warnings = append(warnings, "less than 50% of workloads have owner annotations")
			}
			if coverage.Tier < 0.5 {
				warnings = append(warnings, "less than 50% of workloads have tier annotations — routing will default to standard")
			}
			if coverage.GitOpsRepo < 0.3 {
				warnings = append(warnings, "less than 30% of workloads have gitops-repo — agents cannot create PRs")
			}

			output := DoctorOutput{
				K8sConnectivity: "ok",
				TotalWorkloads:  total,
				Coverage:        coverage,
				AgentReadiness:  readiness,
				Warnings:        warnings,
			}

			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Printf("K8s connectivity: OK\n")
			fmt.Printf("Total workloads:  %d\n", total)
			fmt.Printf("Agent readiness:  %.0f%%\n\n", readiness*100)
			fmt.Printf("Annotation coverage:\n")
			fmt.Printf("  owner:       %.0f%%\n", coverage.Owner*100)
			fmt.Printf("  tier:        %.0f%%\n", coverage.Tier*100)
			fmt.Printf("  gitops-repo: %.0f%%\n", coverage.GitOpsRepo*100)
			fmt.Printf("  oncall:      %.0f%%\n", coverage.Oncall*100)
			fmt.Printf("  runbook:     %.0f%%\n", coverage.Runbook*100)
			fmt.Printf("  depends-on:  %.0f%%\n", coverage.DependsOn*100)

			if len(warnings) > 0 {
				fmt.Println("\nWarnings:")
				for _, w := range warnings {
					fmt.Printf("  ⚠ %s\n", w)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "Output format: text, json")
	return cmd
}

func computeCoverage(services []k8s.ServiceStatus) AnnotationCoverage {
	total := float64(len(services))
	if total == 0 {
		return AnnotationCoverage{}
	}

	var owner, tier, gitops, oncall, runbook, depends float64
	for _, svc := range services {
		if svc.Owner != nil {
			owner++
		}
		if svc.Tier != nil {
			tier++
		}
		if svc.Integration.GitOpsRepo != nil {
			gitops++
		}
		if svc.Integration.Oncall != nil {
			oncall++
		}
		if svc.Integration.Runbook != nil {
			runbook++
		}
		if len(svc.DependsOn) > 0 {
			depends++
		}
	}

	return AnnotationCoverage{
		Owner:      owner / total,
		Tier:       tier / total,
		GitOpsRepo: gitops / total,
		Oncall:     oncall / total,
		Runbook:    runbook / total,
		DependsOn:  depends / total,
	}
}
