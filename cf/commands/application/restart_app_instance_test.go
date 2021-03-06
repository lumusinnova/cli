package application_test

import (
	"errors"

	"github.com/cloudfoundry/cli/cf/api/appinstances/appinstancesfakes"
	"github.com/cloudfoundry/cli/cf/configuration/coreconfig"
	"github.com/cloudfoundry/cli/cf/models"
	testcmd "github.com/cloudfoundry/cli/testhelpers/commands"
	testconfig "github.com/cloudfoundry/cli/testhelpers/configuration"
	testreq "github.com/cloudfoundry/cli/testhelpers/requirements"
	testterm "github.com/cloudfoundry/cli/testhelpers/terminal"

	"github.com/cloudfoundry/cli/cf/commandregistry"
	. "github.com/cloudfoundry/cli/testhelpers/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("restart-app-instance", func() {
	var (
		ui                  *testterm.FakeUI
		config              coreconfig.Repository
		appInstancesRepo    *appinstancesfakes.FakeAppInstancesRepository
		requirementsFactory *testreq.FakeReqFactory
		application         models.Application
		deps                commandregistry.Dependency
	)

	BeforeEach(func() {
		application = models.Application{}
		application.Name = "my-app"
		application.GUID = "my-app-guid"
		application.InstanceCount = 1

		ui = &testterm.FakeUI{}
		appInstancesRepo = new(appinstancesfakes.FakeAppInstancesRepository)
		config = testconfig.NewRepositoryWithDefaults()
		requirementsFactory = &testreq.FakeReqFactory{
			LoginSuccess:         true,
			TargetedSpaceSuccess: true,
			Application:          application,
		}
	})

	updateCommandDependency := func(pluginCall bool) {
		deps.UI = ui
		deps.Config = config
		deps.RepoLocator = deps.RepoLocator.SetAppInstancesRepository(appInstancesRepo)
		commandregistry.Commands.SetCommand(commandregistry.Commands.FindCommand("restart-app-instance").SetDependency(deps, pluginCall))
	}

	runCommand := func(args ...string) bool {
		return testcmd.RunCLICommand("restart-app-instance", args, requirementsFactory, updateCommandDependency, false, ui)
	}

	Describe("requirements", func() {
		It("fails if not logged in", func() {
			requirementsFactory.LoginSuccess = false
			Expect(runCommand("my-app", "0")).To(BeFalse())
		})

		It("fails if a space is not targeted", func() {
			requirementsFactory.TargetedSpaceSuccess = false
			Expect(runCommand("my-app", "0")).To(BeFalse())
		})

		It("fails when there is not exactly two arguments", func() {
			Expect(runCommand("my-app")).To(BeFalse())
			Expect(runCommand("my-app", "0", "0")).To(BeFalse())
			Expect(runCommand()).To(BeFalse())
		})
	})

	Describe("restarting an instance of an application", func() {
		It("correctly 'restarts' the desired instance", func() {
			runCommand("my-app", "0")

			app_guid, instance := appInstancesRepo.DeleteInstanceArgsForCall(0)
			Expect(app_guid).To(Equal(application.GUID))
			Expect(instance).To(Equal(0))
			Expect(ui.Outputs).To(ContainSubstrings(
				[]string{"Restarting instance 0 of application my-app as my-user"},
				[]string{"OK"},
			))
		})

		Context("when deleting the app instance fails", func() {
			BeforeEach(func() {
				appInstancesRepo.DeleteInstanceReturns(errors.New("deletion failed"))
			})
			It("fails", func() {
				runCommand("my-app", "0")

				app_guid, instance := appInstancesRepo.DeleteInstanceArgsForCall(0)
				Expect(app_guid).To(Equal(application.GUID))
				Expect(instance).To(Equal(0))

				Expect(ui.Outputs).To(ContainSubstrings(
					[]string{"FAILED"},
					[]string{"deletion failed"},
				))
			})
		})

		Context("when the instance passed is not an non-negative integer", func() {
			It("fails when it is a string", func() {
				runCommand("my-app", "some-silly-thing")

				Expect(ui.Outputs).To(ContainSubstrings(
					[]string{"Instance must be a non-negative integer"},
				))
			})
		})
	})
})
