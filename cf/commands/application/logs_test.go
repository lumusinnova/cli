package application_test

import (
	"time"

	"github.com/cloudfoundry/cli/cf/api/logs"
	"github.com/cloudfoundry/cli/cf/api/logs/logsfakes"
	"github.com/cloudfoundry/cli/cf/commandregistry"
	"github.com/cloudfoundry/cli/cf/errors"
	"github.com/cloudfoundry/cli/cf/models"
	testcmd "github.com/cloudfoundry/cli/testhelpers/commands"
	testconfig "github.com/cloudfoundry/cli/testhelpers/configuration"
	testlogs "github.com/cloudfoundry/cli/testhelpers/logs"
	testreq "github.com/cloudfoundry/cli/testhelpers/requirements"
	testterm "github.com/cloudfoundry/cli/testhelpers/terminal"
	"github.com/cloudfoundry/loggregatorlib/logmessage"

	"github.com/cloudfoundry/cli/cf/configuration/coreconfig"
	. "github.com/cloudfoundry/cli/testhelpers/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("logs command", func() {
	var (
		ui                  *testterm.FakeUI
		logsRepo            *logsfakes.FakeRepository
		requirementsFactory *testreq.FakeReqFactory
		configRepo          coreconfig.Repository
		deps                commandregistry.Dependency
	)

	updateCommandDependency := func(pluginCall bool) {
		deps.UI = ui
		deps.RepoLocator = deps.RepoLocator.SetLogsRepository(logsRepo)
		deps.Config = configRepo
		commandregistry.Commands.SetCommand(commandregistry.Commands.FindCommand("logs").SetDependency(deps, pluginCall))
	}

	BeforeEach(func() {
		ui = &testterm.FakeUI{}
		configRepo = testconfig.NewRepositoryWithDefaults()
		logsRepo = new(logsfakes.FakeRepository)
		requirementsFactory = &testreq.FakeReqFactory{}
	})

	runCommand := func(args ...string) bool {
		return testcmd.RunCLICommand("logs", args, requirementsFactory, updateCommandDependency, false, ui)
	}

	Describe("requirements", func() {
		It("fails with usage when called without one argument", func() {
			requirementsFactory.LoginSuccess = true

			runCommand()
			Expect(ui.Outputs).To(ContainSubstrings(
				[]string{"Incorrect Usage", "Requires an argument"},
			))
		})

		It("fails requirements when not logged in", func() {
			Expect(runCommand("my-app")).To(BeFalse())
		})
		It("fails if a space is not targeted", func() {
			requirementsFactory.LoginSuccess = true
			requirementsFactory.TargetedSpaceSuccess = false
			Expect(runCommand("--recent", "my-app")).To(BeFalse())
		})

	})

	Context("when logged in", func() {
		var (
			app models.Application
		)

		BeforeEach(func() {
			requirementsFactory.LoginSuccess = true
			requirementsFactory.TargetedSpaceSuccess = true

			app = models.Application{}
			app.Name = "my-app"
			app.GUID = "my-app-guid"

			currentTime := time.Now()
			recentLogs := []logs.Loggable{
				testlogs.NewLogMessage("Log Line 1", app.GUID, "DEA", "1", logmessage.LogMessage_ERR, currentTime),
				testlogs.NewLogMessage("Log Line 2", app.GUID, "DEA", "1", logmessage.LogMessage_ERR, currentTime),
			}

			appLogs := []logs.Loggable{
				testlogs.NewLogMessage("Log Line 1", app.GUID, "DEA", "1", logmessage.LogMessage_ERR, time.Now()),
			}

			requirementsFactory.Application = app
			logsRepo.RecentLogsForReturns(recentLogs, nil)
			logsRepo.TailLogsForStub = func(appGUID string, onConnect func(), logChan chan<- logs.Loggable, errChan chan<- error) {
				onConnect()
				go func() {
					for _, log := range appLogs {
						logChan <- log
					}
					close(logChan)
					close(errChan)
				}()
			}
		})

		It("shows the recent logs when the --recent flag is provided", func() {
			runCommand("--recent", "my-app")

			Expect(requirementsFactory.ApplicationName).To(Equal("my-app"))
			Expect(app.GUID).To(Equal(logsRepo.RecentLogsForArgsForCall(0)))
			Expect(ui.Outputs).To(ContainSubstrings(
				[]string{"Connected, dumping recent logs for app", "my-app", "my-org", "my-space", "my-user"},
				[]string{"Log Line 1"},
				[]string{"Log Line 2"},
			))
		})

		Context("when the log messages contain format string identifiers", func() {
			BeforeEach(func() {
				logsRepo.RecentLogsForReturns([]logs.Loggable{
					testlogs.NewLogMessage("hello%2Bworld%v", app.GUID, "DEA", "1", logmessage.LogMessage_ERR, time.Now()),
				}, nil)
			})

			It("does not treat them as format strings", func() {
				runCommand("--recent", "my-app")
				Expect(ui.Outputs).To(ContainSubstrings([]string{"hello%2Bworld%v"}))
			})
		})

		It("tails the app's logs when no flags are given", func() {
			runCommand("my-app")

			Expect(requirementsFactory.ApplicationName).To(Equal("my-app"))
			appGUID, _, _, _ := logsRepo.TailLogsForArgsForCall(0)
			Expect(app.GUID).To(Equal(appGUID))
			Expect(ui.Outputs).To(ContainSubstrings(
				[]string{"Connected, tailing logs for app", "my-app", "my-org", "my-space", "my-user"},
				[]string{"Log Line 1"},
			))
		})

		Context("when the loggregator server has an invalid cert", func() {
			Context("when the skip-ssl-validation flag is not set", func() {
				It("fails and informs the user about the skip-ssl-validation flag", func() {
					logsRepo.TailLogsForStub = func(appGUID string, onConnect func(), logChan chan<- logs.Loggable, errChan chan<- error) {
						errChan <- errors.NewInvalidSSLCert("https://example.com", "it don't work good")
					}
					runCommand("my-app")

					Expect(ui.Outputs).To(ContainSubstrings(
						[]string{"Received invalid SSL certificate", "https://example.com"},
						[]string{"TIP"},
					))
				})

				It("informs the user of the error when they include the --recent flag", func() {
					logsRepo.RecentLogsForReturns(nil, errors.NewInvalidSSLCert("https://example.com", "how does SSL work???"))
					runCommand("--recent", "my-app")

					Expect(ui.Outputs).To(ContainSubstrings(
						[]string{"Received invalid SSL certificate", "https://example.com"},
						[]string{"TIP"},
					))
				})
			})
		})

		Context("when the loggregator server has a valid cert", func() {
			It("tails logs", func() {
				runCommand("my-app")
				Expect(ui.Outputs).To(ContainSubstrings(
					[]string{"Connected, tailing logs for app", "my-org", "my-space", "my-user"},
				))
			})
		})
	})
})
