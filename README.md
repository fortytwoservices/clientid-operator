# ClientID Operator for Azure Managed Identities

## Overview

The ClientID Operator synchronizes Azure managed identities, service accounts, and role assignments in a Kubernetes environment. This operator ensures that Azure service principals and client IDs are correctly applied to Kubernetes service accounts and role assignments based on specified naming conventions and labels/annotations.

## Prerequisites

This operator works with specific kinds and versions of resources:
- **Managed Identities**: `apiVersion: managedidentity.azure.upbound.io/v1beta1 and managedidentity.azure.m.upbound.io/v1beta1, Kind: UserAssignedIdentity`
- **Service Accounts**: `apiVersion: v1, Kind: ServiceAccount`
- **Role Assignments**: `apiVersion: authorization.azure.upbound.io/v1beta1 and roleassignments.authorization.azure.m.upbound.io/v1beta1, Kind: RoleAssignment`

## Naming Syntax

To ensure proper synchronization, resources must follow a strict naming syntax:
- **Managed Identities** should be named with the prefix and the application name, e.g. `workload-identity-{appName}`.
- **Service Accounts** should follow a similar naming convention, e.g., `workload-identity-{appName}`.
- **Role Assignments** should use a naming convention that includes the application name, e.g., `ra-service-{appName}-dv-azunea-contributor`.

## Labels and Annotations

Proper annotations and labels are crucial for the operator to function correctly:
- **Service Accounts** must include the `azure.workload.identity/client-id` annotation.
- **Role Assignments** must have the correct labels: 
  - `application: {appName}`
  - `type: roleassignment`

These labels allow the operator to identify and process the correct Role Assignment resources associated with the respective Managed Identity.

## Usage

Deploy the operator in your Kubernetes cluster, ensuring that all managed resources conform to the naming syntax and label requirements outlined above. The operator will automatically update the annotations on Service Accounts and the principal ID in Role Assignments based on changes to the corresponding Managed Identities.

## Contributing

Contributions to this project are welcome! Please ensure that any submitted issues or pull requests adhere to the naming conventions and resource specifications outlined in this document.
