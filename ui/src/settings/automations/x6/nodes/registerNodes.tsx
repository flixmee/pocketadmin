import { Graph, Node } from "@antv/x6";
import type { ReactElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import type { WorkflowNodeRenderData } from "../types";
import { ActionNodeView, ApprovalNodeView, PlaceholderNodeView, TriggerNodeView } from "./components";

export const WORKFLOW_NODE_SHAPES = {
    trigger: "workflow-trigger-node",
    action: "workflow-action-node",
    approval: "workflow-approval-node",
    placeholder: "workflow-placeholder-node",
} as const;

const mountedRoots = new WeakMap<HTMLElement, Root>();
let isRegistered = false;

function mountReactNode(
    cell: Node,
    Component: (props: { node: WorkflowNodeRenderData }) => ReactElement,
) {
    const container = document.createElement("div");
    container.style.width = "100%";
    container.style.height = "100%";
    container.style.pointerEvents = "auto";

    const root = createRoot(container);
    mountedRoots.set(container, root);
    root.render(<Component node={cell.getData<WorkflowNodeRenderData>()} />);

    // X6 owns the DOM node lifecycle. This observer gives React a chance to
    // release event handlers when the HTML node leaves the graph container.
    const observer = new MutationObserver(() => {
        if (!container.isConnected) {
            observer.disconnect();
            mountedRoots.get(container)?.unmount();
            mountedRoots.delete(container);
        }
    });

    queueMicrotask(() => {
        if (document.body.contains(container)) {
            observer.observe(document.body, { childList: true, subtree: true });
        }
    });

    return container;
}

export function registerWorkflowNodes() {
    if (isRegistered) {
        return;
    }

    Graph.registerNode(
        WORKFLOW_NODE_SHAPES.trigger,
        {
            inherit: "html",
            width: 300,
            height: 72,
            html(cell: Node) {
                return mountReactNode(cell, TriggerNodeView);
            },
        },
        true,
    );

    Graph.registerNode(
        WORKFLOW_NODE_SHAPES.action,
        {
            inherit: "html",
            width: 320,
            height: 118,
            html(cell: Node) {
                return mountReactNode(cell, ActionNodeView);
            },
        },
        true,
    );

    Graph.registerNode(
        WORKFLOW_NODE_SHAPES.approval,
        {
            inherit: "html",
            width: 320,
            height: 146,
            html(cell: Node) {
                return mountReactNode(cell, ApprovalNodeView);
            },
        },
        true,
    );

    Graph.registerNode(
        WORKFLOW_NODE_SHAPES.placeholder,
        {
            inherit: "html",
            width: 260,
            height: 92,
            html(cell: Node) {
                return mountReactNode(cell, PlaceholderNodeView);
            },
        },
        true,
    );

    isRegistered = true;
}
