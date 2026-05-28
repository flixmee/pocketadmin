import type { Graph } from "@antv/x6";

export type NodeType = "trigger" | "action" | "approval" | "placeholder";

export type BranchKey = "approved" | "rejected";

export interface WorkflowNode {
    id: string;
    type: NodeType;
    title: string;
    subtitle: string;
    icon: string;
    stepNumber?: number;
    isValid?: boolean;
    branches?: {
        approved: WorkflowNode[];
        rejected: WorkflowNode[];
    };
}

export interface WorkflowState {
    nodes: WorkflowNode[];
}

export interface ActionDefinition {
    type: string;
    title: string;
    subtitle: string;
    icon: string;
    nodeType?: Extract<NodeType, "action" | "approval">;
}

export interface AddNodeInput {
    id?: string;
    title?: string;
    subtitle?: string;
    icon?: string;
    stepNumber?: number;
    isValid?: boolean;
    branch?: BranchKey;
    afterNodeId?: string;
}

export interface WorkflowGraphHandle {
    addNode: (type: string, data?: AddNodeInput) => WorkflowNode;
    removeNode: (id: string) => void;
    moveNode: (id: string, position: { x: number; y: number }) => void;
    getGraph: () => Graph | null;
}

export interface WorkflowGraphProps {
    value?: WorkflowState;
    defaultValue?: WorkflowState;
    selectedNodeId?: string;
    onChange?: (nextState: WorkflowState) => void;
    onSelectNode?: (node: WorkflowNode | null) => void;
    onAddStep?: (sourceNodeId: string, branch?: BranchKey) => void;
    onEditNode?: (nodeId: string) => void;
    onDeleteNode?: (nodeId: string) => void;
}

export interface WorkflowNodeRenderData extends WorkflowNode {
    branch?: BranchKey;
    placeholderText?: string;
    selected?: boolean;
    deletable?: boolean;
    onEdit?: (nodeId: string) => void;
    onDelete?: (nodeId: string) => void;
}

export interface EdgeButton {
    id: string;
    sourceNodeId: string;
    branch?: BranchKey;
    x: number;
    y: number;
}

export interface LayoutNode {
    node: WorkflowNodeRenderData;
    x: number;
    y: number;
    width: number;
    height: number;
    parentId?: string;
    branch?: BranchKey;
}

export interface LayoutEdge {
    id: string;
    source: string;
    target: string;
    sourceNodeId: string;
    branch?: BranchKey;
    vertices?: Array<{ x: number; y: number }>;
    label?: string;
    addButton?: boolean;
}
