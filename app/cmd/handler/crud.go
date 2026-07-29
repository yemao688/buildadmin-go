package handler

import (
	"fmt"
	"go-build-admin/app/middleware"
	helper "go-build-admin/app/pkg/crud_helper"
	"go-build-admin/conf"
	"go-build-admin/service/db"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CrudHandler struct {
	logger *zap.Logger
	config *conf.Configuration
	db     *gorm.DB
}

func NewCrudHandler(logger *zap.Logger, config *conf.Configuration) *CrudHandler {
	return &CrudHandler{
		logger: logger, config: config, db: db.NewDB(config, logger),
	}
}

func (h *CrudHandler) Generate(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: crud:generate <spec.yaml>")
	}
	opts, err := helper.LoadSpec(args[0])
	if err != nil {
		return fmt.Errorf("load CRUD spec error: %w", err)
	}
	opts.SkipMenu, _ = cmd.Flags().GetBool("skip-menu")
	opts.AdminID, _ = cmd.Flags().GetInt32("admin-id")
	opts.RegisterAtomicRoute = func(method, path string) {
		action := path[strings.LastIndex(path, "/")+1:]
		middleware.RegisterAtomicRoute(middleware.AtomicRoute{Route: path[:strings.LastIndex(path, "/")], Action: action, Method: method})
	}
	opts.UnregisterAtomicRoute = func(method, path string) {
		action := path[strings.LastIndex(path, "/")+1:]
		middleware.UnregisterAtomicRoute(middleware.AtomicRoute{Route: path[:strings.LastIndex(path, "/")], Action: action, Method: method})
	}
	result, err := helper.GenerateFromSpec(h.db, h.config, *opts)
	if err != nil {
		return fmt.Errorf("CRUD generation error: %w", err)
	}
	cmd.Printf("CRUD generation success (log id: %d)\n", result.LogID)
	for _, file := range result.Files {
		cmd.Printf("%s\n", file)
	}
	return nil
}

func (h *CrudHandler) Apply(cmd *cobra.Command, args []string) error {
	opts := helper.ApplyOptions{AdminID: 1}
	opts.AllowRebuild, _ = cmd.Flags().GetBool("allow-rebuild")
	opts.SkipMenu, _ = cmd.Flags().GetBool("skip-menu")
	opts.AdminID, _ = cmd.Flags().GetInt32("admin-id")
	var results []helper.ApplyTableResult
	var err error
	if len(args) == 0 {
		dir := helper.DefaultSpecDir()
		if dir == "" {
			return fmt.Errorf("no spec directory crud_specs found; pass spec files explicitly: crud:apply <spec.yaml...>")
		}
		results, err = helper.ApplySpecsFromDir(h.db, h.config, dir, opts)
	} else {
		results, err = helper.ApplySpecs(h.db, h.config, args, opts)
	}
	if err != nil {
		return fmt.Errorf("CRUD apply error: %w", err)
	}
	if len(results) == 0 {
		cmd.Println("CRUD apply: no specs found, nothing to do")
		return nil
	}
	for _, result := range results {
		line := fmt.Sprintf("CRUD apply %-9s %s (log id: %d)", string(result.Action), result.Table, result.LogID)
		if len(result.Changes) > 0 {
			line += ": " + strings.Join(result.Changes, ", ")
		}
		cmd.Println(line)
	}
	return nil
}

func (h *CrudHandler) Delete(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: crud:delete <tableName>")
	}
	if err := helper.DeleteFromSpecWithHooks(h.db, h.config, args[0], func(method, path string) {
		action := path[strings.LastIndex(path, "/")+1:]
		middleware.UnregisterAtomicRoute(middleware.AtomicRoute{Route: path[:strings.LastIndex(path, "/")], Action: action, Method: method})
	}); err != nil {
		return fmt.Errorf("CRUD deletion error: %w", err)
	}
	cmd.Printf("CRUD deletion success: %s\n", args[0])
	return nil
}
