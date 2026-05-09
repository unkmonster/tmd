package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/unkmonster/tmd/internal/database"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("tmd-db-migrate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dbPath := fs.String("db", "", "path to foo.db")
	fromRoot := fs.String("from-root", "", "existing download root recorded in parent_dir values")
	toRoot := fs.String("to-root", "", "new download root to rewrite parent_dir values to")
	dryRun := fs.Bool("dry-run", false, "report changes without writing the database")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s -db <foo.db> -from-root <old-root> -to-root <new-root> [-dry-run]\n", fs.Name())
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dbPath == "" || *fromRoot == "" || *toRoot == "" {
		fs.Usage()
		return 2
	}

	result, err := database.MigrateParentDirsInSQLiteFile(*dbPath, database.ParentDirMigrationOptions{
		FromRoot: *fromRoot,
		ToRoot:   *toRoot,
		DryRun:   *dryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		return 1
	}

	fmt.Printf("dry_run=%t\n", *dryRun)
	fmt.Printf("user_entities: %d/%d updated\n", result.UserEntitiesUpdated, result.UserEntitiesTotal)
	fmt.Printf("lst_entities: %d/%d updated\n", result.LstEntitiesUpdated, result.LstEntitiesTotal)
	if result.BackupPath != "" {
		fmt.Printf("backup=%s\n", result.BackupPath)
	}
	if len(result.Samples) > 0 {
		fmt.Println("samples:")
		for _, sample := range result.Samples {
			fmt.Printf("- %s id=%d: %q -> %q\n", sample.Table, sample.ID, sample.From, sample.To)
		}
	}
	if result.UserEntitiesUpdated == 0 && result.LstEntitiesUpdated == 0 {
		fmt.Println("no changes required")
	}

	return 0
}
