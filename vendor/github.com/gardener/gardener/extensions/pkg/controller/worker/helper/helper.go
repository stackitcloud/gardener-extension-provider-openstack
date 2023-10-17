// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper

import (
	"fmt"
	"regexp"
	"strings"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/hashicorp/go-multierror"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
)

const (
	nameLabel = "name"
	// MachineSetKind is the kind of the owner reference of a machine set
	MachineSetKind = "MachineSet"
	// MachineDeploymentKind is the kind of the owner reference of a machine deployment
	MachineDeploymentKind = "MachineDeployment"
)

// TODO(nschad): Remove with Gardner v1.76+
// SEE: https://dev.azure.com/schwarzit/schwarzit.ske/_workitems/edit/536673
var (
	unauthenticatedRegexp               = regexp.MustCompile(`(?i)(Authentication failed|invalid character|invalid_client|cannot fetch token|InvalidSecretAccessKey)`)
	unauthorizedRegexp                  = regexp.MustCompile(`(?i)(Unauthorized|SignatureDoesNotMatch|invalid_grant|Authorization Profile was not found|no active subscriptions|not authorized|AccessDenied|PolicyNotAuthorized)`)
	quotaExceededRegexp                 = regexp.MustCompile(`(?i)((?:^|[^t]|(?:[^s]|^)t|(?:[^e]|^)st|(?:[^u]|^)est|(?:[^q]|^)uest|(?:[^e]|^)quest|(?:[^r]|^)equest)LimitExceeded|Quotas|Quota.*exceeded|exceeded quota|Quota has been met|QUOTA_EXCEEDED|Maximum number of ports exceeded|VolumeSizeExceedsAvailableQuota)`)
	rateLimitsExceededRegexp            = regexp.MustCompile(`(?i)(RequestLimitExceeded|Throttling|Too many requests)`)
	dependenciesRegexp                  = regexp.MustCompile(`(?i)(PendingVerification|Access Not Configured|accessNotConfigured|DependencyViolation|OptInRequired|Conflict|inactive billing state|timeout while waiting for state to become|InvalidCidrBlock|already busy for|internal server error|A resource with the ID|There are not enough hosts available)`)
	retryableDependenciesRegexp         = regexp.MustCompile(`(?i)(RetryableError)`)
	resourcesDepletedRegexp             = regexp.MustCompile(`(?i)(not available in the current hardware cluster|out of stock)`)
	configurationProblemRegexp          = regexp.MustCompile(`(?i)(not supported in your requested Availability Zone|notFound|Invalid value|violates constraint|no attached internet gateway found|Your query returned no results|invalid VPC attributes|unrecognized feature gate|runtime-config invalid key|strict decoder error|not allowed to configure an unsupported|error during apply of object .* is invalid:|duplicate zones|overlapping zones)`)
	retryableConfigurationProblemRegexp = regexp.MustCompile(`(?i)(is misconfigured and requires zero voluntary evictions|SDK.CanNotResolveEndpoint|The requested configuration is currently not supported)`)

	// KnownCodes maps Gardener error codes to respective regex.
	KnownCodes = map[gardencorev1beta1.ErrorCode]func(string) bool{
		gardencorev1beta1.ErrorInfraUnauthenticated:          unauthenticatedRegexp.MatchString,
		gardencorev1beta1.ErrorInfraUnauthorized:             unauthorizedRegexp.MatchString,
		gardencorev1beta1.ErrorInfraQuotaExceeded:            quotaExceededRegexp.MatchString,
		gardencorev1beta1.ErrorInfraRateLimitsExceeded:       rateLimitsExceededRegexp.MatchString,
		gardencorev1beta1.ErrorInfraDependencies:             dependenciesRegexp.MatchString,
		gardencorev1beta1.ErrorRetryableInfraDependencies:    retryableDependenciesRegexp.MatchString,
		gardencorev1beta1.ErrorInfraResourcesDepleted:        resourcesDepletedRegexp.MatchString,
		gardencorev1beta1.ErrorConfigurationProblem:          configurationProblemRegexp.MatchString,
		gardencorev1beta1.ErrorRetryableConfigurationProblem: retryableConfigurationProblemRegexp.MatchString,
	}
)

// BuildOwnerToMachinesMap builds a map from a slice of machinev1alpha1.Machine, that maps the owner reference
// to a slice of machines with the same owner reference
func BuildOwnerToMachinesMap(machines []machinev1alpha1.Machine) map[string][]machinev1alpha1.Machine {
	ownerToMachines := make(map[string][]machinev1alpha1.Machine)
	for index, machine := range machines {
		if len(machine.OwnerReferences) > 0 {
			for _, reference := range machine.OwnerReferences {
				if reference.Kind == MachineSetKind {
					ownerToMachines[reference.Name] = append(ownerToMachines[reference.Name], machines[index])
				}
			}
		} else if len(machine.Labels) > 0 {
			if machineDeploymentName, ok := machine.Labels[nameLabel]; ok {
				ownerToMachines[machineDeploymentName] = append(ownerToMachines[machineDeploymentName], machines[index])
			}
		}
	}
	return ownerToMachines
}

// BuildOwnerToMachineSetsMap builds a map from a slice of machinev1alpha1.MachineSet, that maps the owner reference
// to a slice of MachineSets with the same owner reference
func BuildOwnerToMachineSetsMap(machineSets []machinev1alpha1.MachineSet) map[string][]machinev1alpha1.MachineSet {
	ownerToMachineSets := make(map[string][]machinev1alpha1.MachineSet)
	for index, machineSet := range machineSets {
		if len(machineSet.OwnerReferences) > 0 {
			for _, reference := range machineSet.OwnerReferences {
				if reference.Kind == MachineDeploymentKind {
					ownerToMachineSets[reference.Name] = append(ownerToMachineSets[reference.Name], machineSets[index])
				}
			}
		} else if len(machineSet.Labels) > 0 {
			if machineDeploymentName, ok := machineSet.Labels[nameLabel]; ok {
				ownerToMachineSets[machineDeploymentName] = append(ownerToMachineSets[machineDeploymentName], machineSets[index])
			}
		}
	}
	return ownerToMachineSets
}

// GetMachineSetWithMachineClass checks if for the given <machineDeploymentName>, there exists a machine set in the <ownerReferenceToMachineSet> with the machine class <machineClassName>
// returns the machine set or nil
func GetMachineSetWithMachineClass(machineDeploymentName, machineClassName string, ownerReferenceToMachineSet map[string][]machinev1alpha1.MachineSet) *machinev1alpha1.MachineSet {
	machineSets := ownerReferenceToMachineSet[machineDeploymentName]
	for _, machineSet := range machineSets {
		if machineSet.Spec.Template.Spec.Class.Name == machineClassName {
			return &machineSet
		}
	}
	return nil
}

// ReportFailedMachines reports the names of failed machines in the given `status` per error description.
func ReportFailedMachines(status machinev1alpha1.MachineDeploymentStatus) error {
	machines := status.FailedMachines
	if len(machines) == 0 {
		return nil
	}

	descriptionPerFailedMachines := make(map[string][]string)
	for _, machine := range machines {
		descriptionPerFailedMachines[machine.LastOperation.Description] = append(descriptionPerFailedMachines[machine.LastOperation.Description],
			fmt.Sprintf("%q", machine.Name))
	}

	allErrs := &multierror.Error{
		ErrorFormat: errorsutils.NewErrorFormatFuncWithPrefix("machine(s) failed"),
	}
	for description, names := range descriptionPerFailedMachines {
		allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %s", strings.Join(names, ", "), description))
	}

	return allErrs
}
