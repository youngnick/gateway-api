/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resourcediscovery

import (
	"fmt"
	"maps"
	"sort"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	"sigs.k8s.io/gateway-api/gwctl/pkg/policymanager"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

// ResourceModel represents the graph-like model of Gateway API resources and
// their relationships, capturing dependencies and interactions. It acts as a
// central data structure for organizing and managing these resources, enabling
// operations like:
//   - Discovering and understanding the relationships between different
//     resources
//   - Calculating effective policies based on hierarchical inheritance
//   - Identifying potential conflicts or issues in resource configuration
//   - Visualizing the topology of Gateway API resources
type ResourceModel struct {
	GatewayClasses  map[gatewayClassID]*GatewayClassNode
	Namespaces      map[namespaceID]*NamespaceNode
	Gateways        map[gatewayID]*GatewayNode
	HTTPRoutes      map[httpRouteID]*HTTPRouteNode
	Backends        map[backendID]*BackendNode
	ReferenceGrants map[referenceGrantID]*ReferenceGrantNode
	Policies        map[policyID]*PolicyNode
}

// addGatewayClasses adds nodes for GatewayClases.
func (rm *ResourceModel) addGatewayClasses(gatewayClasses ...gatewayv1.GatewayClass) {
	if rm.GatewayClasses == nil {
		rm.GatewayClasses = make(map[gatewayClassID]*GatewayClassNode)
	}
	for _, gatewayClass := range gatewayClasses {
		gatewayClassNode := NewGatewayClassNode(&gatewayClass)
		if _, ok := rm.GatewayClasses[gatewayClassNode.ID()]; !ok {
			rm.GatewayClasses[gatewayClassNode.ID()] = gatewayClassNode
		}
	}
}

// addNamespace adds nodes for Namespace.
func (rm *ResourceModel) addNamespace(namespaces ...corev1.Namespace) {
	if rm.Namespaces == nil {
		rm.Namespaces = make(map[namespaceID]*NamespaceNode)
	}
	for _, namespace := range namespaces {
		namespaceNode := NewNamespaceNode(namespace)
		if _, ok := rm.Namespaces[namespaceNode.ID()]; !ok {
			rm.Namespaces[namespaceNode.ID()] = namespaceNode
		}
	}
}

// addGateways adds nodes for Gateways.
func (rm *ResourceModel) addGateways(gateways ...gatewayv1.Gateway) {
	if rm.Gateways == nil {
		rm.Gateways = make(map[gatewayID]*GatewayNode)
	}
	for _, gateway := range gateways {
		gatewayNode := NewGatewayNode(&gateway)
		if _, ok := rm.Gateways[gatewayNode.ID()]; !ok {
			rm.Gateways[gatewayNode.ID()] = gatewayNode
		}
	}
}

// addHTTPRoutes adds nodes for HTTPRoutes.
func (rm *ResourceModel) addHTTPRoutes(httpRoutes ...gatewayv1.HTTPRoute) {
	if rm.HTTPRoutes == nil {
		rm.HTTPRoutes = make(map[httpRouteID]*HTTPRouteNode)
	}
	for _, httpRoute := range httpRoutes {
		httpRouteNode := NewHTTPRouteNode(&httpRoute)
		if _, ok := rm.HTTPRoutes[httpRouteNode.ID()]; !ok {
			rm.HTTPRoutes[httpRouteNode.ID()] = httpRouteNode
		}
	}
}

// addBackends adds nodes for Backends.
func (rm *ResourceModel) addBackends(backends ...unstructured.Unstructured) {
	if rm.Backends == nil {
		rm.Backends = make(map[backendID]*BackendNode)
	}
	for _, backend := range backends {
		backendNode := NewBackendNode(&backend)
		if _, ok := rm.Backends[backendNode.ID()]; !ok {
			rm.Backends[backendNode.ID()] = backendNode
		}
	}
}

// addReferenceGrants adds nodes for ReferenceGrants.
func (rm *ResourceModel) addReferenceGrants(referenceGrants ...gatewayv1beta1.ReferenceGrant) {
	if rm.ReferenceGrants == nil {
		rm.ReferenceGrants = make(map[referenceGrantID]*ReferenceGrantNode)
	}
	for _, referenceGrant := range referenceGrants {
		referenceGrantNode := NewReferenceGrantNode(&referenceGrant)
		if _, ok := rm.ReferenceGrants[referenceGrantNode.ID()]; !ok {
			rm.ReferenceGrants[referenceGrantNode.ID()] = referenceGrantNode
		}
	}
}

// addPolicyIfTargetExists adds a node for Policy only if the target for the
// Policy exists in the ResourceModel. In addition to adding the Node, it also
// makes the connections with the targetRefs.
func (rm *ResourceModel) addPolicyIfTargetExists(policies ...policymanager.Policy) {
	if rm.Policies == nil {
		rm.Policies = make(map[policyID]*PolicyNode)
	}
	for _, policy := range policies {
		policyNode := NewPolicyNode(&policy)

		switch {
		case policy.TargetRef().Group == gatewayv1.GroupName:
			switch policy.TargetRef().Kind {
			case "GatewayClass":
				gwcID := GatewayClassID(policy.TargetRef().Name)
				gatewayClassNode, ok := rm.GatewayClasses[gwcID]
				if !ok {
					klog.V(1).ErrorS(nil, "Skipping policy since targetRef GatewayClass does not exist in ResourceModel", "policy", policy.Name(), "gatewayClassID", gwcID)
					continue
				}
				rm.Policies[policyNode.ID()] = policyNode
				policyNode.GatewayClass = gatewayClassNode
				gatewayClassNode.Policies[policyNode.ID()] = policyNode

			case "Gateway":
				gwID := GatewayID(policy.TargetRef().Namespace, policy.TargetRef().Name)
				gatewayNode, ok := rm.Gateways[gwID]
				if !ok {
					klog.V(1).ErrorS(nil, "Skipping policy since targetRef Gateway does not exist in ResourceModel", "policy", policy.Name(), "gatewayID", gwID)
					continue
				}
				rm.Policies[policyNode.ID()] = policyNode
				policyNode.Gateway = gatewayNode
				gatewayNode.Policies[policyNode.ID()] = policyNode

			case "HTTPRoute":
				hrID := HTTPRouteID(policy.TargetRef().Namespace, policy.TargetRef().Name)
				httpRouteNode, ok := rm.HTTPRoutes[hrID]
				if !ok {
					klog.V(1).ErrorS(nil, "Skipping policy since targetRef HTTPRoute does not exist in ResourceModel", "policy", policy.Name(), "httpRouteID", hrID)
					continue
				}
				rm.Policies[policyNode.ID()] = policyNode
				policyNode.HTTPRoute = httpRouteNode
				httpRouteNode.Policies[policyNode.ID()] = policyNode
			}

		case policy.TargetRef().Group == corev1.GroupName && policy.TargetRef().Kind == "Namespace":
			nsID := NamespaceID(policy.TargetRef().Name)
			namespaceNode, ok := rm.Namespaces[nsID]
			if !ok {
				klog.V(1).ErrorS(nil, "Skipping policy since targetRef Namespace does not exist in ResourceModel", "policy", policy.Name(), "namespaceID", nsID)
				continue
			}
			rm.Policies[policyNode.ID()] = policyNode
			policyNode.Namespace = namespaceNode
			namespaceNode.Policies[policyNode.ID()] = policyNode

		default: // Assume attached to backend and evaluate further.
			bID := BackendID(policy.TargetRef().Group, policy.TargetRef().Kind, policy.TargetRef().Namespace, policy.TargetRef().Name)
			backendNode, ok := rm.Backends[bID]
			if !ok {
				klog.V(1).ErrorS(nil, "Skipping policy since targetRef Backend does not exist in ResourceModel", "policy", policy.Name(), "backendID", bID)
				continue
			}
			rm.Policies[policyNode.ID()] = policyNode
			policyNode.Backend = backendNode
			backendNode.Policies[policyNode.ID()] = policyNode
		}
	}
}

// connectGatewayWithGatewayClass establishes a connection between a Gateway and
// its associated GatewayClass.
func (rm *ResourceModel) connectGatewayWithGatewayClass(gatewayID gatewayID, gatewayClassID gatewayClassID) {
	gatewayNode, ok := rm.Gateways[gatewayID]
	if !ok {
		klog.V(1).ErrorS(nil, "Gateway does not exist in ResourceModel", "gatewayID", gatewayID)
		return
	}
	gatewayClassNode, ok := rm.GatewayClasses[gatewayClassID]
	if !ok {
		klog.V(1).ErrorS(nil, "GatewayClass does not exist in ResourceModel", "gatewayClassID", gatewayClassID)
		return
	}

	gatewayNode.GatewayClass = gatewayClassNode
	gatewayClassNode.Gateways[gatewayID] = gatewayNode
}

// connectHTTPRouteWithGateway establishes a connection between an HTTPRoute and
// its parent Gateway.
func (rm *ResourceModel) connectHTTPRouteWithGateway(httpRouteID httpRouteID, gatewayID gatewayID) {
	httpRouteNode, ok := rm.HTTPRoutes[httpRouteID]
	if !ok {
		klog.V(1).ErrorS(nil, "HTTPRoute does not exist in ResourceModel", "httpRouteID", httpRouteID)
		return
	}
	gatewayNode, ok := rm.Gateways[gatewayID]
	if !ok {
		klog.V(1).ErrorS(nil, "Gateway does not exist in ResourceModel", "gatewayID", gatewayID)
		return
	}

	httpRouteNode.Gateways[gatewayID] = gatewayNode
	gatewayNode.HTTPRoutes[httpRouteID] = httpRouteNode
}

// connectHTTPRouteWithBackend establishes a connection between an HTTPRoute and
// its targeted Backend.
func (rm *ResourceModel) connectHTTPRouteWithBackend(httpRouteID httpRouteID, backendID backendID) {
	httpRouteNode, ok := rm.HTTPRoutes[httpRouteID]
	if !ok {
		klog.V(1).ErrorS(nil, "HTTPRoute does not exist in ResourceModel", "httpRouteID", httpRouteID)
		return
	}
	backendNode, ok := rm.Backends[backendID]
	if !ok {
		klog.V(1).ErrorS(nil, "Backend does not exist in ResourceModel", "backendID", backendID)
		return
	}

	httpRouteNode.Backends[backendID] = backendNode
	backendNode.HTTPRoutes[httpRouteID] = httpRouteNode
}

// connectGatewayWithNamespace establishes a connection between a Gateway and
// its Namespace.
func (rm *ResourceModel) connectGatewayWithNamespace(gatewayID gatewayID, namespaceID namespaceID) {
	gatewayNode, ok := rm.Gateways[gatewayID]
	if !ok {
		klog.V(1).ErrorS(nil, "Gateway does not exist in ResourceModel", "gatewayID", gatewayID)
		return
	}
	namespaceNode, ok := rm.Namespaces[namespaceID]
	if !ok {
		klog.V(1).ErrorS(nil, "Namespace does not exist in ResourceModel", "namespaceID", namespaceID)
		return
	}

	gatewayNode.Namespace = namespaceNode
	namespaceNode.Gateways[gatewayID] = gatewayNode
}

// connectHTTPRouteWithNamespace establishes a connection between an HTTPRoute
// and its Namespace.
func (rm *ResourceModel) connectHTTPRouteWithNamespace(httpRouteID httpRouteID, namespaceID namespaceID) {
	httpRouteNode, ok := rm.HTTPRoutes[httpRouteID]
	if !ok {
		klog.V(1).ErrorS(nil, "HTTPRoute does not exist in ResourceModel", "httpRouteID", httpRouteID)
		return
	}
	namespaceNode, ok := rm.Namespaces[namespaceID]
	if !ok {
		klog.V(1).ErrorS(nil, "Namespace does not exist in ResourceModel", "namespaceID", namespaceID)
		return
	}

	httpRouteNode.Namespace = namespaceNode
	namespaceNode.HTTPRoutes[httpRouteID] = httpRouteNode
}

// connectBackendWithNamespace establishes a connection between a Backend and
// its Namespace.
func (rm *ResourceModel) connectBackendWithNamespace(backendID backendID, namespaceID namespaceID) {
	backendNode, ok := rm.Backends[backendID]
	if !ok {
		klog.V(1).ErrorS(nil, "Backend does not exist in ResourceModel", "backendID", backendID)
		return
	}
	namespaceNode, ok := rm.Namespaces[namespaceID]
	if !ok {
		klog.V(1).ErrorS(nil, "Namespace does not exist in ResourceModel", "namespaceID", namespaceID)
		return
	}

	backendNode.Namespace = namespaceNode
	namespaceNode.Backends[backendID] = backendNode
}

// connectReferenceGrantWithBackend establishes a connection between a ReferenceGrant and
// a Backend.
func (rm *ResourceModel) connectReferenceGrantWithBackend(referenceGrantID referenceGrantID, backendID backendID) {
	referenceGrantNode, ok := rm.ReferenceGrants[referenceGrantID]
	if !ok {
		klog.V(1).ErrorS(nil, "ReferenceGrant does not exist in ResourceModel", "referenceGrantID", referenceGrantID)
		return
	}
	backendNode, ok := rm.Backends[backendID]
	if !ok {
		klog.V(1).ErrorS(nil, "Backend does not exist in ResourceModel", "backendID", backendID)
		return
	}

	referenceGrantNode.Backends[backendID] = backendNode
	backendNode.ReferenceGrants[referenceGrantID] = referenceGrantNode
}

// calculateEffectivePolicies calculates the effective policies for all
// Gateways, HTTPRoutes, and Backends in the ResourceModel.
func (rm *ResourceModel) calculateEffectivePolicies() error {
	if err := rm.calculateEffectivePoliciesForGateways(); err != nil {
		return err
	}
	if err := rm.calculateEffectivePoliciesForHTTPRoutes(); err != nil {
		return err
	}
	if err := rm.calculateEffectivePoliciesForBackends(); err != nil {
		return err
	}
	return nil
}

// calculateEffectivePoliciesForGateways calculates the effective policies for
// each Gateway by merging policies from different hierarchies (GatewayClass,
// Namespace, and Gateway).
func (rm *ResourceModel) calculateEffectivePoliciesForGateways() error {
	for _, gatewayNode := range rm.Gateways {
		// Do not calculate effective policy for the Gateway if the referenced
		// GatewayClass does not exist. For now, we only calculate effective policy
		// once the references are corrected.
		if gatewayNode.GatewayClass == nil {
			continue
		}

		// Fetch all policies.
		gatewayClassPolicies := convertPoliciesMapToSlice(gatewayNode.GatewayClass.Policies)
		gatewayNamespacePolicies := convertPoliciesMapToSlice(gatewayNode.Namespace.Policies)
		gatewayPolicies := convertPoliciesMapToSlice(gatewayNode.Policies)

		// Merge policies by their kind.
		gatewayClassPoliciesByKind, err := policymanager.MergePoliciesOfSimilarKind(gatewayClassPolicies)
		if err != nil {
			return err
		}
		gatewayNamespacePoliciesByKind, err := policymanager.MergePoliciesOfSimilarKind(gatewayNamespacePolicies)
		if err != nil {
			return err
		}
		gatewayPoliciesByKind, err := policymanager.MergePoliciesOfSimilarKind(gatewayPolicies)
		if err != nil {
			return err
		}

		// Merge all hierarchial policies.
		result, err := policymanager.MergePoliciesOfDifferentHierarchy(gatewayClassPoliciesByKind, gatewayNamespacePoliciesByKind)
		if err != nil {
			return err
		}

		result, err = policymanager.MergePoliciesOfDifferentHierarchy(result, gatewayPoliciesByKind)
		if err != nil {
			return err
		}

		gatewayNode.EffectivePolicies = result
	}
	return nil
}

// calculateEffectivePoliciesForHTTPRoutes calculates the effective policies for
// each HTTPRoute, taking into account policies from different hierarchies
// (GatewayClass, Namespace, Gateway, and HTTPRoute).
func (rm *ResourceModel) calculateEffectivePoliciesForHTTPRoutes() error {
	for _, httpRouteNode := range rm.HTTPRoutes {
		result := make(map[gatewayID]map[policymanager.PolicyCrdID]policymanager.Policy)

		// Step 1: Aggregate all policies of the HTTPRoute and the
		// HTTPRoute-namespace.
		httpRoutePolicies := convertPoliciesMapToSlice(httpRouteNode.Policies)
		httpRouteNamespacePolicies := convertPoliciesMapToSlice(httpRouteNode.Namespace.Policies)

		// Step 2: Merge HTTPRoute and HTTPRoute-namespace policies by their kind.
		httpRoutePoliciesByKind, err := policymanager.MergePoliciesOfSimilarKind(httpRoutePolicies)
		if err != nil {
			return err
		}
		httpRouteNamespacePoliciesByKind, err := policymanager.MergePoliciesOfSimilarKind(httpRouteNamespacePolicies)
		if err != nil {
			return err
		}

		// Step 3: Loop through all Gateways and merge policies for each Gateway.
		// End result is we get policies partitioned by each Gateway.
		for gatewayID, gatewayNode := range httpRouteNode.Gateways {
			gatewayPoliciesByKind := gatewayNode.EffectivePolicies

			// Merge all hierarchial policies.
			mergedPolicies, err := policymanager.MergePoliciesOfDifferentHierarchy(gatewayPoliciesByKind, httpRouteNamespacePoliciesByKind)
			if err != nil {
				return err
			}

			mergedPolicies, err = policymanager.MergePoliciesOfDifferentHierarchy(mergedPolicies, httpRoutePoliciesByKind)
			if err != nil {
				return err
			}

			result[gatewayID] = mergedPolicies
		}

		httpRouteNode.EffectivePolicies = result
	}
	return nil
}

// calculateEffectivePoliciesForBackends calculates the effective policies for
// each Backend, considering policies from different hierarchies (GatewayClass,
// Namespace, Gateway, HTTPRoute, and Backend).
func (rm *ResourceModel) calculateEffectivePoliciesForBackends() error {
	for _, backendNode := range rm.Backends {
		result := make(map[gatewayID]map[policymanager.PolicyCrdID]policymanager.Policy)

		// Step 1: Aggregate all policies of the Backend and the Backend-namespace.
		backendPolicies := convertPoliciesMapToSlice(backendNode.Policies)
		backendNamespacePolicies := convertPoliciesMapToSlice(backendNode.Namespace.Policies)

		// Step 2: Merge Backend and Backend-namespace policies by their kind.
		backendPoliciesByKind, err := policymanager.MergePoliciesOfSimilarKind(backendPolicies)
		if err != nil {
			return err
		}
		backendNamespacePoliciesByKind, err := policymanager.MergePoliciesOfSimilarKind(backendNamespacePolicies)
		if err != nil {
			return err
		}

		// Step 3: Loop through all HTTPRoutes and get their effective policies. Merge
		// effective policies such that we get policies partitioned by Gateway.
		for _, httpRouteNode := range backendNode.HTTPRoutes {
			httpRoutePoliciesByGateway := httpRouteNode.EffectivePolicies

			for gatewayID, policies := range httpRoutePoliciesByGateway {
				result[gatewayID], err = policymanager.MergePoliciesOfSameHierarchy(result[gatewayID], policies)
				if err != nil {
					return err
				}
			}
		}

		// Step 4: Loop through all Gateways and merge the Backend and
		// Backend-namespace specific policies. Note that this needs to be done
		// separately from Step 4 i.e. we can't have this loop within Step 4 itself.
		// This is because we first want to merge all policies of the same-hierarchy
		// together and then move to the next hierarchy of Backend and
		// Backend-namespace.
		for gatewayID := range result {
			// Merge all hierarchial policies.
			result[gatewayID], err = policymanager.MergePoliciesOfDifferentHierarchy(result[gatewayID], backendNamespacePoliciesByKind)
			if err != nil {
				return err
			}

			result[gatewayID], err = policymanager.MergePoliciesOfDifferentHierarchy(result[gatewayID], backendPoliciesByKind)
			if err != nil {
				return err
			}
		}

		backendNode.EffectivePolicies = result
	}
	return nil
}

func convertPoliciesMapToSlice(policies map[policyID]*PolicyNode) []policymanager.Policy {
	var result []policymanager.Policy
	for _, policyNode := range policies {
		result = append(result, *policyNode.Policy)
	}
	sort.Slice(result, func(i, j int) bool {
		a := fmt.Sprintf("%v/%v/%v", result[i].PolicyCrdID(), result[i].Unstructured().GetNamespace(), result[i].Unstructured().GetName())
		b := fmt.Sprintf("%v/%v/%v", result[j].PolicyCrdID(), result[j].Unstructured().GetNamespace(), result[j].Unstructured().GetName())
		return a < b
	})
	return result
}

// calculateInheritedPolicies calculates the inherited polices for all
// Gateways, HTTRoutes, and Backends in ResourceModel.
func (rm *ResourceModel) calculateInheritedPolicies() error {
	if err := rm.calculateInheritedPoliciesForGateways(); err != nil {
		return err
	}
	if err := rm.calculateInheritedPoliciesForHTTPRoutes(); err != nil {
		return err
	}
	if err := rm.calculateInheritedPoliciesForBackends(); err != nil {
		return err
	}
	return nil
}

// calculateInheritedPoliciesForGateways calculates the inherited policies for
// all Gateways present in ResourceModel.
func (rm *ResourceModel) calculateInheritedPoliciesForGateways() error {
	for _, gatewayNode := range rm.Gateways {
		result := make(map[policyID]*PolicyNode)

		// Policies inherited from Gateway's namespace.
		policiesInheritedFromNamespace := filterInheritablePolicies(gatewayNode.Namespace.Policies)
		maps.Copy(result, policiesInheritedFromNamespace)

		// Policies inherited from GatewayClass.
		if gatewayNode.GatewayClass != nil {
			policiesInheritedFromGatewayClass := filterInheritablePolicies(gatewayNode.GatewayClass.Policies)
			maps.Copy(result, policiesInheritedFromGatewayClass)
		}

		gatewayNode.InheritedPolicies = result
	}
	return nil
}

// calculateInheritedPoliciesForHTTPRoutes calculates the inherited policies for
// all HTTPRoutes present in ResourceModel.
func (rm *ResourceModel) calculateInheritedPoliciesForHTTPRoutes() error {
	for _, httpRouteNode := range rm.HTTPRoutes {
		result := make(map[policyID]*PolicyNode)

		// Policies inherited from HTTPRoute's namespace.
		policiesInheritedFromNamespace := filterInheritablePolicies(httpRouteNode.Namespace.Policies)
		maps.Copy(result, policiesInheritedFromNamespace)

		// Policies inherited from Gateways.
		for _, gatewayNode := range httpRouteNode.Gateways {
			// Add policies inherited by GatewayNode.
			maps.Copy(result, gatewayNode.InheritedPolicies)

			// Add inheritable policies directly applied to GatewayNode.
			maps.Copy(result, filterInheritablePolicies(gatewayNode.Policies))
		}

		httpRouteNode.InheritedPolicies = result
	}
	return nil
}

// calculateInheritedPoliciesForBackends calculates the inherited policies for
// all Backends present in ResourceModel.
func (rm *ResourceModel) calculateInheritedPoliciesForBackends() error {
	for _, backendNode := range rm.Backends {
		result := make(map[policyID]*PolicyNode)

		// Policies inherited from Backend's namespace.
		policiesInheritedFromNamespace := filterInheritablePolicies(backendNode.Namespace.Policies)
		maps.Copy(result, policiesInheritedFromNamespace)

		// Policies inherited from HTTPRoutes.
		for _, httpRouteNode := range backendNode.HTTPRoutes {
			// Add policies inherited by HTTPRouteNode.
			maps.Copy(result, httpRouteNode.InheritedPolicies)

			// Add inheritable policies directly applied to HTTPRouteNode.
			maps.Copy(result, filterInheritablePolicies(httpRouteNode.Policies))
		}

		backendNode.InheritedPolicies = result
	}
	return nil
}

// filterInheritablePolicies filters and returns policies which can be inherited.
func filterInheritablePolicies(policies map[policyID]*PolicyNode) map[policyID]*PolicyNode {
	inheritablePolicies := make(map[policyID]*PolicyNode)

	for policyID, policyNode := range policies {
		if policyNode.Policy.IsInherited() {
			inheritablePolicies[policyID] = policyNode
		}
	}

	return inheritablePolicies
}
