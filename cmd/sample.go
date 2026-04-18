package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const arazzoTemplate = `arazzo: 1.0.1
info:
  title: Pet Upsert Workflow (V3)
  summary: A sample workflow that conditionally creates or updates a pet using Petstore V3
  description: Workflow targeting Petstore V3 API. Takes an id and name - renames the pet if it exists, creates it if not.
  version: 1.0.0

sourceDescriptions:
  - name: petstoreApiV3
    url: https://petstore3.swagger.io/api/v3/openapi.json
    type: openapi

workflows:
  - workflowId: ensurePetExistsV3
    summary: Check if a pet exists by ID; update its name if found, create it if not.
    inputs:
      type: object
      properties:
        petId: { type: integer, default: 12345 }
        newName: { type: string, default: Fluffy }

    steps:
      - stepId: checkStep
        description: Check if the pet exists and route accordingly.
        operationId: getPetById
        parameters:
          - name: petId
            in: path
            value: $inputs.petId

        # Both 200 and 404 are expected/valid responses - treat both as success
        successCriteria:
          - condition: $statusCode == 200
          - condition: $statusCode == 404

        # Branch based on which status code was returned
        onSuccess:
          - name: petFoundRouteToUpdate
            criteria:
              - condition: $statusCode == 200
            type: goto
            stepId: updateStep
          - name: petMissingRouteToCreate
            criteria:
              - condition: $statusCode == 404
            type: goto
            stepId: createStep

        # Retry on true server errors
        onFailure:
          - name: retryOnServerError
            criteria:
              - condition: $statusCode >= 500
            type: retry
            retryAfter: 5

      - stepId: createStep
        description: Pet not found - create it with the given id and name.
        operationId: addPet
        requestBody:
          contentType: application/json
          payload:
            id: $inputs.petId
            name: $inputs.newName
            category:
              id: 1
              name: Dogs
            photoUrls: 
              - "https://example.com/pet.jpg"
            tags:
              - id: 0
                name: string
            status: "available"
        onSuccess:
          - name: endAfterCreation
            type: end

      - stepId: updateStep
        description: Pet found - rename it using a full PUT update.
        operationId: updatePet
        requestBody:
          contentType: application/json
          payload:
            id: $inputs.petId
            name: $inputs.newName
            category:
              id: 1
              name: Dogs
            photoUrls: 
              - "https://example.com/pet.jpg"
            tags:
              - id: 0
                name: string
            status: "available"
        onSuccess:
          - name: endAfterUpdate
            type: end
`

var sampleOutput string

var sampleCmd = &cobra.Command{
	Use:   "sample [project-name]",
	Short: "Generate a sample Arazzo project",
	Long:  "Creates a new directory with a sample Arazzo workflow file configured for the Petstore v3 API.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		folderName := "sample-arazzo-project"
		if sampleOutput != "" {
			folderName = sampleOutput
		} else if len(args) > 0 {
			folderName = args[0]
		}

		// Create the directory
		err := os.MkdirAll(folderName, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Create the sample arazzo file
		filePath := filepath.Join(folderName, "petstore_workflow.yaml")
		err = os.WriteFile(filePath, []byte(arazzoTemplate), 0644)
		if err != nil {
			return fmt.Errorf("failed to write sample arazzo file: %w", err)
		}

		fmt.Printf("Successfully created sample project in '%s'\n", folderName)
		fmt.Printf("Sample Arazzo file created at: %s\n", filePath)
		return nil
	},
}

func init() {
	sampleCmd.Flags().StringVarP(&sampleOutput, "output", "o", "", "Output directory name to create the sample project in")
	rootCmd.AddCommand(sampleCmd)
}
