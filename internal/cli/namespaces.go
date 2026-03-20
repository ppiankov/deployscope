package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ppiankov/deployscope/internal/k8s"
)

type NamespaceInfo struct {
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Owners  []string `json:"owners,omitempty"`
	Healthy int      `json:"healthy"`
	Down    int      `json:"down"`
}

func newNamespacesCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "namespaces",
		Short: "Show namespace summary with ownership",
		RunE: func(cmd *cobra.Command, args []string) error {
			k8sClient, err := k8s.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			services, _, err := k8sClient.FetchDeployments(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to fetch workloads: %w", err)
			}

			nsMap := map[string]*NamespaceInfo{}
			for _, svc := range services {
				ns, ok := nsMap[svc.Namespace]
				if !ok {
					ns = &NamespaceInfo{Name: svc.Namespace}
					nsMap[svc.Namespace] = ns
				}
				ns.Count++
				switch svc.Status {
				case "green":
					ns.Healthy++
				case "red":
					ns.Down++
				}
				if svc.Owner != nil {
					found := false
					for _, o := range ns.Owners {
						if o == *svc.Owner {
							found = true
							break
						}
					}
					if !found {
						ns.Owners = append(ns.Owners, *svc.Owner)
					}
				}
			}

			var namespaces []NamespaceInfo
			for _, ns := range nsMap {
				namespaces = append(namespaces, *ns)
			}
			sort.Slice(namespaces, func(i, j int) bool {
				return namespaces[i].Name < namespaces[j].Name
			})

			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(namespaces)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAMESPACE\tWORKLOADS\tHEALTHY\tDOWN\tOWNERS")
			for _, ns := range namespaces {
				owners := "-"
				if len(ns.Owners) > 0 {
					owners = fmt.Sprintf("%v", ns.Owners)
				}
				_, _ = fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\n",
					ns.Name, ns.Count, ns.Healthy, ns.Down, owners)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json")
	return cmd
}
