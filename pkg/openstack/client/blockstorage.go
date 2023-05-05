// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	utilsvolumes "github.com/gophercloud/utils/openstack/blockstorage/v3/volumes"
)

const (
	// VolumeStatusAvailable indicates that he volume is available to be attached.
	VolumeStatusAvailable = "available"
	// VolumeStatusCreating indicates that the volume is being created.
	VolumeStatusCreating = "creating"
	// VolumeStatusDownloading indicates that the volume is in downloading state.
	VolumeStatusDownloading = "downloading"
	// VolumeStatusDeleting indicates that the volume is in the process of being deleted.
	VolumeStatusDeleting = "deleting"
	// VolumeStatusError indicates that the volume is in error state.
	VolumeStatusError = "error"
	// VolumeStatusInUse indicates that the volume is currently in use.
	VolumeStatusInUse = "in-use"
)

// BlockStorageClient is a client for the Cinder service.
type BlockStorageClient struct {
	client *gophercloud.ServiceClient
}

// GetVolume retrieves information about a volume.
func (b *BlockStorageClient) GetVolume(id string) (*volumes.Volume, error) {
	return volumes.Get(b.client, id).Extract()
}

// VolumeIDFromName resolves the given volume name to a unique ID.
func (b *BlockStorageClient) VolumeIDFromName(name string) (string, error) {
	return utilsvolumes.IDFromName(b.client, name)
}

// CreateVolume creates a volume.
func (b *BlockStorageClient) CreateVolume(opts volumes.CreateOptsBuilder) (*volumes.Volume, error) {
	return volumes.Create(b.client, opts).Extract()
}

// DeleteVolume deletes a volume.
func (b *BlockStorageClient) DeleteVolume(id string) error {
	return volumes.Delete(b.client, id, volumes.DeleteOpts{}).ExtractErr()
}
