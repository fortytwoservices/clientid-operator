# clientid-operator

The clientid operator runs in your kubernetes cluster. It constantly looks for changes made to the API version: managedidentity.azure.upbound.io/v1beta1 and Kind: UserAssignedIdentity

It will sync the client id from the status field of the user assigned managed identity in Azure to your service account azure.workload.identity/client-id annotation, ensuring the service account is using the managed identity for auth.
