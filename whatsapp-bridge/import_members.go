package main

import (
	"context"
	"fmt"
	"log"

	"whatsapp-client/config"
	"whatsapp-client/domain"
	"whatsapp-client/repository"
)

func importMembersFromCSVToPostgres(configPath string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return err
	}

	db, err := repository.OpenPostgresFromEnv()
	if err != nil {
		return err
	}
	defer db.Close()

	store := repository.NewPostgresMemberStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		return err
	}

	csvRepo := repository.NewCSVRepository()

	totalGroups := 0
	totalMembers := 0

	for sevaType, gl := range cfg.Groups {
		for _, g := range gl {
			members, err := csvRepo.ReadMembers(g.CSVPath)
			if err != nil {
				return fmt.Errorf("failed to read csv for %s group %d (%s): %w", sevaType, g.Number, g.CSVPath, err)
			}

			_, currentVersion, err := store.GetGroupMembers(domain.SevaType(sevaType), g.Number)
			if err != nil {
				return fmt.Errorf("failed to get current version for %s group %d: %w", sevaType, g.Number, err)
			}

			_, err = store.ReplaceGroupMembers(domain.SevaType(sevaType), g.Number, members, currentVersion)
			if err != nil {
				return fmt.Errorf("failed to replace members for %s group %d: %w", sevaType, g.Number, err)
			}

			totalGroups++
			totalMembers += len(members)
			log.Printf("Imported %s group %d: %d members", sevaType, g.Number, len(members))
		}
	}

	log.Printf("Import complete: %d groups, %d members", totalGroups, totalMembers)
	return nil
}
