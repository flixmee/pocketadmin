import { useMemo, useRef, useState } from "react";
import { Sidebar } from "./Sidebar";
import type { ActionDefinition, BranchKey, WorkflowGraphHandle, WorkflowNode, WorkflowState } from "./types";
import { WorkflowGraph } from "./WorkflowGraph";

export const initialWorkflow: WorkflowState = {
    nodes: [
        {
            id: "trigger",
            type: "trigger",
            title: "Trigger",
            subtitle: "record.create",
            icon: "ti-bolt",
        },
        {
            id: "step-1",
            type: "approval",
            title: "Wait approval",
            subtitle: "Approval by hungtranqt93@gmail.com",
            icon: "ti-user-check",
            stepNumber: 1,
            isValid: true,
            branches: { approved: [], rejected: [] },
        },
        {
            id: "step-2",
            type: "action",
            title: "AI generate",
            subtitle: "Use Please help me write a content with the title ...",
            icon: "ti-sparkles",
            stepNumber: 2,
            isValid: true,
        },
        {
            id: "step-3",
            type: "action",
            title: "Update record",
            subtitle: "Update record in pbc_4268781541",
            icon: "ti-edit",
            stepNumber: 3,
            isValid: true,
        },
    ],
};

export interface WorkflowBuilderProps {
    value?: WorkflowState;
    defaultValue?: WorkflowState;
    onChange?: (state: WorkflowState) => void;
    onAddStep?: (sourceNodeId: string, branch?: BranchKey) => void;
    onEditNode?: (nodeId: string) => void;
    onSelectNode?: (node: WorkflowNode | null) => void;
}

function countSteps(nodes: WorkflowNode[]) {
    let count = 0;

    nodes.forEach((node) => {
        if (node.type !== "trigger" && node.type !== "placeholder") {
            count += 1;
        }
        if (node.branches) {
            count += countSteps(node.branches.approved);
            count += countSteps(node.branches.rejected);
        }
    });

    return count;
}

export function WorkflowBuilder(props: WorkflowBuilderProps) {
    const graphRef = useRef<WorkflowGraphHandle | null>(null);
    const [internalState, setInternalState] = useState<WorkflowState>(() => props.defaultValue || initialWorkflow);
    const [selectedNode, setSelectedNode] = useState<WorkflowNode | null>(null);
    const workflow = props.value || internalState;
    const stepCount = useMemo(() => countSteps(workflow.nodes), [workflow]);

    function handleChange(nextState: WorkflowState) {
        if (!props.value) {
            setInternalState(nextState);
        }
        props.onChange?.(nextState);
    }

    function handleAddAction(action: ActionDefinition) {
        graphRef.current?.addNode(action.type, {
            title: action.title,
            subtitle: action.subtitle,
            icon: action.icon,
            isValid: false,
        });
    }

    function handleDeleteNode(nodeId: string) {
        graphRef.current?.removeNode(nodeId);
    }

    return (
        <div className="flex h-full min-h-[720px] w-full overflow-hidden bg-slate-50">
            <Sidebar onAddAction={handleAddAction} />
            <main className="relative min-w-0 flex-1">
                <div className="absolute right-4 top-4 z-10 rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 shadow-sm">
                    {stepCount}/100 steps
                </div>
                {selectedNode && (
                    <div className="absolute left-4 top-4 z-10 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-600 shadow-sm">
                        <span className="font-semibold text-slate-950">{selectedNode.title}</span>
                        <span className="ml-2 text-slate-400">{selectedNode.subtitle}</span>
                    </div>
                )}
                <WorkflowGraph
                    ref={graphRef}
                    value={workflow}
                    onChange={handleChange}
                    onSelectNode={(node) => {
                        setSelectedNode(node);
                        props.onSelectNode?.(node);
                    }}
                    onAddStep={props.onAddStep}
                    onEditNode={props.onEditNode}
                    onDeleteNode={handleDeleteNode}
                />
            </main>
        </div>
    );
}
