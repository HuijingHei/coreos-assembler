// Copyright 2023 Red Hat
// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"github.com/coreos/coreos-assembler/mantle/platform"
)

type Options struct {
	*platform.Options

	AzureCredentials  string
	AzureSubscription string

	DiskURI           string
	Publisher         string
	Offer             string
	Sku               string
	Version           string
	Size              string
	Location          string
	AvailabilityZone  string
	ManagedIdentityID string

	SubscriptionName string
	SubscriptionID   string

	// Azure Storage API endpoint suffix. If unset, the Azure SDK default will be used.
	StorageEndpointSuffix string
}
