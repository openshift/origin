## Integration Tests
Runnning integration tests will create real IBM Power Cloud resources.

 - `internal/testutils/integration_utils.go`: 	 	Defines commonly used test functions
 - `internal/testutils/integration_variables.go`: Lists default values for all test variables
 - `internal/testutils/example.env`:              An example .env which is used when overriding default test variables
 - `internal/testutils/launch.json`:              An example test configuration
 - `client/instance/*_integration_test.go`:       Defines tests for an individual resource type

### Setup
To setup the integration tests, you will neet to create a `.vscode` directory in the project root. Copy `.env` and `launch.json` into `.vscode`. Default integration test values are provided, however, you can define custom  variables in `.env`. You can  also create additional `.env` files (`staging.env`, `production.env`, `dal12.env`, ...) for different environments, regions, resources, etc. If you create a different `.env` file, modify `launch.json` to include a testing configuration for that file.

Every Integration Test requires these variables:
 - `API_KEY` or `BEARER_TOKEN` (deprecated but currently working)
 - `CRN`
A default `CRN` is provided, but you will need to define your own `API_KEY` or `BEARER_TOKEN`. It is recomended that you do this in `.env`.
### Test Steps
Each integration test runs through these steps:
 - `init()` in `integration_utils.go` is called. This first checks to see any environment variables are defined in `.env`. If an environment variable isn't defined, the default value in `integration_variables.go` is used.
 - A precheck function (ex. `ImagePreCheck()`) is called. This verifies that all required variables for the preCheck's test are defined and loaded properly.
 - The test runs
 
### Run a test 
To run a test:
 - Set `DisableTesting = False` in `integration_variables.go`.
 - Double click the test function name to select the text.
 - Click `Run and Debug` on the VScode sidebar, and then click `Run a test`. Output will be visible in the VScode Debug Console.
   - `launch.json` runs tests by using this selected text. Feel free to create your own `launch.json` configuration.

### Updating
When updating or creating new integration tests:
 - Add test variable definitions to `integration_variables.go`.
 - Update `init()` and relevant `PreCheck()` functions in `integration_utils.go`
 - Make sure to add a way for newly provisioned resources to be deleted at the end of the test