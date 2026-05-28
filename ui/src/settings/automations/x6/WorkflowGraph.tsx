import { Graph } from "@antv/x6";
import {
    type DragEvent,
    forwardRef,
    useCallback,
    useEffect,
    useImperativeHandle,
    useMemo,
    useRef,
    useState,
} from "react";
import { registerWorkflowNodes, WORKFLOW_NODE_SHAPES } from "./nodes/registerNodes";
import type {
    AddNodeInput,
    BranchKey,
    EdgeButton,
    LayoutEdge,
    LayoutNode,
    WorkflowGraphHandle,
    WorkflowGraphProps,
    WorkflowNode,
    WorkflowNodeRenderData,
    WorkflowState,
} from "./types";

const nodeSize = {
    trigger: { width: 300, height: 72 },
    action: { width: 320, height: 118 },
    approval: { width: 320, height: 146 },
    placeholder: { width: 260, height: 92 },
};

const rootCenterX = 520;
const branchOffsetX = 230;
const topPadding = 48;
const stepGapY = 54;
const branchGapY = 78;

const defaultActionMeta = {
    condition: { title: "Condition", subtitle: "Branch when a rule matches", icon: "ti-git-merge" },
    code: { title: "Code", subtitle: "Run a JavaScript snippet", icon: "ti-code" },
    http: { title: "HTTP request", subtitle: "Call an external endpoint", icon: "ti-world" },
    "mail.send": { title: "Send mail", subtitle: "Send an email message", icon: "ti-mail" },
    "record.create": { title: "Create record", subtitle: "Create a PocketBase record", icon: "ti-plus" },
    "record.update": { title: "Update record", subtitle: "Update a PocketBase record", icon: "ti-edit" },
    "record.delete": { title: "Delete record", subtitle: "Delete a PocketBase record", icon: "ti-trash" },
    capability: { title: "Capability", subtitle: "Run a configured capability", icon: "ti-puzzle" },
    "wait.delay": { title: "Wait delay", subtitle: "Pause this workflow", icon: "ti-clock" },
    "wait.webhook": { title: "Wait webhook", subtitle: "Resume from a webhook", icon: "ti-webhook" },
    "wait.event": { title: "Wait event", subtitle: "Resume from an event", icon: "ti-radar" },
    "wait.approval": { title: "Wait approval", subtitle: "Pause until a decision is made", icon: "ti-user-check" },
    "ai.generate": { title: "AI generate", subtitle: "Generate text with AI", icon: "ti-sparkles" },
    "ai.extract": { title: "AI extract", subtitle: "Extract structured data", icon: "ti-scan" },
    "ai.classify": { title: "AI classify", subtitle: "Classify content with AI", icon: "ti-category" },
} as const;

interface LayoutResult {
    nodes: LayoutNode[];
    edges: LayoutEdge[];
}

interface SequenceLayoutResult {
    firstIds: string[];
    endIds: string[];
    endY: number;
}

type MutableWorkflowState = WorkflowState & { nodes: WorkflowNode[] };

function cloneWorkflow(state: WorkflowState): MutableWorkflowState {
    return {
        nodes: state.nodes.map(cloneWorkflowNode),
    };
}

function cloneWorkflowNode(node: WorkflowNode): WorkflowNode {
    return {
        ...node,
        branches: node.branches
            ? {
                approved: node.branches.approved.map(cloneWorkflowNode),
                rejected: node.branches.rejected.map(cloneWorkflowNode),
            }
            : undefined,
    };
}

function actionCount(nodes: WorkflowNode[]) {
    let count = 0;

    walkNodes(nodes, (node) => {
        if (node.type !== "trigger" && node.type !== "placeholder") {
            count += 1;
        }
    });

    return count;
}

function walkNodes(nodes: WorkflowNode[], visit: (node: WorkflowNode) => void) {
    nodes.forEach((node) => {
        visit(node);
        if (node.branches) {
            walkNodes(node.branches.approved, visit);
            walkNodes(node.branches.rejected, visit);
        }
    });
}

function renumberSteps(state: WorkflowState) {
    let step = 1;
    const next = cloneWorkflow(state);

    walkNodes(next.nodes, (node) => {
        if (node.type !== "trigger" && node.type !== "placeholder") {
            node.stepNumber = step;
            step += 1;
        }
    });

    return next;
}

function findWorkflowNode(nodes: WorkflowNode[], id: string): WorkflowNode | null {
    for (const node of nodes) {
        if (node.id === id) {
            return node;
        }
        if (node.branches) {
            const nested = findWorkflowNode([...node.branches.approved, ...node.branches.rejected], id);
            if (nested) {
                return nested;
            }
        }
    }
    return null;
}

function createWorkflowNode(type: string, data: AddNodeInput = {}, stepNumber: number): WorkflowNode {
    const meta = defaultActionMeta[type as keyof typeof defaultActionMeta] || {
        title: data.title || type,
        subtitle: "Configure this workflow step",
        icon: data.icon || "ti-bolt",
    };

    const isApproval = type === "approval" || type === "wait.approval";

    return {
        id: data.id || `step-${Date.now()}-${Math.round(Math.random() * 10000)}`,
        type: isApproval ? "approval" : "action",
        title: data.title || meta.title,
        subtitle: data.subtitle || meta.subtitle,
        icon: data.icon || meta.icon,
        stepNumber: data.stepNumber || stepNumber,
        isValid: data.isValid ?? false,
        branches: isApproval ? { approved: [], rejected: [] } : undefined,
    };
}

function insertAfter(nodes: WorkflowNode[], afterNodeId: string, nextNode: WorkflowNode): boolean {
    const index = nodes.findIndex((node) => node.id === afterNodeId);
    if (index >= 0) {
        nodes.splice(index + 1, 0, nextNode);
        return true;
    }

    for (const node of nodes) {
        if (!node.branches) {
            continue;
        }
        if (insertAfter(node.branches.approved, afterNodeId, nextNode)) {
            return true;
        }
        if (insertAfter(node.branches.rejected, afterNodeId, nextNode)) {
            return true;
        }
    }

    return false;
}

function removeNodeById(nodes: WorkflowNode[], id: string): WorkflowNode | null {
    const index = nodes.findIndex((node) => node.id === id);
    if (index >= 0) {
        return nodes.splice(index, 1)[0];
    }

    for (const node of nodes) {
        if (!node.branches) {
            continue;
        }
        const approved = removeNodeById(node.branches.approved, id);
        if (approved) {
            return approved;
        }
        const rejected = removeNodeById(node.branches.rejected, id);
        if (rejected) {
            return rejected;
        }
    }

    return null;
}

function findSiblingList(nodes: WorkflowNode[], id: string): WorkflowNode[] | null {
    if (nodes.some((node) => node.id === id)) {
        return nodes;
    }

    for (const node of nodes) {
        if (!node.branches) {
            continue;
        }
        const approved = findSiblingList(node.branches.approved, id);
        if (approved) {
            return approved;
        }
        const rejected = findSiblingList(node.branches.rejected, id);
        if (rejected) {
            return rejected;
        }
    }

    return null;
}

function branchLabel(branch: BranchKey) {
    return branch === "approved" ? "APPROVED" : "REJECTED";
}

function placeholderTitle(branch: BranchKey) {
    return branch === "approved" ? "No approved steps" : "No rejected steps";
}

function buildLayout(
    state: WorkflowState,
    selectedNodeId: string,
    handlers: Pick<WorkflowGraphProps, "onEditNode" | "onDeleteNode">,
): LayoutResult {
    const nodes: LayoutNode[] = [];
    const edges: LayoutEdge[] = [];

    function addLayoutNode(node: WorkflowNodeRenderData, centerX: number, y: number, branch?: BranchKey) {
        const size = nodeSize[node.type];
        nodes.push({
            node,
            x: centerX - size.width / 2,
            y,
            width: size.width,
            height: size.height,
            branch,
        });
    }

    function connect(
        source: string,
        target: string,
        sourceNodeId = source,
        options: Partial<LayoutEdge> = {},
    ) {
        const id = `${source}--${target}${options.branch ? `--${options.branch}` : ""}`;
        edges.push({
            id,
            source,
            target,
            sourceNodeId,
            branch: options.branch,
            label: options.label,
            vertices: options.vertices,
            addButton: options.addButton ?? true,
        });
    }

    function layoutList(
        list: WorkflowNode[],
        centerX: number,
        startY: number,
        branch?: BranchKey,
    ): SequenceLayoutResult {
        let y = startY;
        const firstIds: string[] = [];
        let pendingEndIds: string[] = [];

        list.forEach((workflowNode) => {
            const renderNode: WorkflowNodeRenderData = {
                ...workflowNode,
                branch,
                selected: workflowNode.id === selectedNodeId,
                deletable: workflowNode.type !== "trigger",
                onEdit: handlers.onEditNode,
                onDelete: handlers.onDeleteNode,
            };

            addLayoutNode(renderNode, centerX, y, branch);

            if (firstIds.length === 0) {
                firstIds.push(workflowNode.id);
            }

            pendingEndIds.forEach((sourceId) => {
                connect(sourceId, workflowNode.id, sourceId, {
                    addButton: pendingEndIds.length === 1,
                });
            });

            if (workflowNode.type === "approval") {
                const branchStartY = y + nodeSize.approval.height + branchGapY;
                const approvedCenterX = centerX - branchOffsetX;
                const rejectedCenterX = centerX + branchOffsetX;
                const approved = layoutBranch(workflowNode, "approved", approvedCenterX, branchStartY);
                const rejected = layoutBranch(workflowNode, "rejected", rejectedCenterX, branchStartY);

                pendingEndIds = [...approved.endIds, ...rejected.endIds];
                y = Math.max(approved.endY, rejected.endY) + stepGapY;
            } else {
                pendingEndIds = [workflowNode.id];
                y += nodeSize[workflowNode.type].height + stepGapY;
            }
        });

        return {
            firstIds,
            endIds: pendingEndIds,
            endY: Math.max(startY, y - stepGapY),
        };
    }

    function layoutBranch(
        parent: WorkflowNode,
        branchKey: BranchKey,
        centerX: number,
        startY: number,
    ): SequenceLayoutResult {
        const branchNodes = parent.branches?.[branchKey] || [];

        if (branchNodes.length === 0) {
            const placeholderId = `${parent.id}__${branchKey}-placeholder`;
            const placeholder: WorkflowNodeRenderData = {
                id: placeholderId,
                type: "placeholder",
                title: placeholderTitle(branchKey),
                subtitle: "",
                icon: "",
                branch: branchKey,
                placeholderText: placeholderTitle(branchKey),
                selected: false,
                deletable: false,
            };

            addLayoutNode(placeholder, centerX, startY, branchKey);
            connect(parent.id, placeholderId, parent.id, {
                branch: branchKey,
                label: branchLabel(branchKey),
            });

            return {
                firstIds: [placeholderId],
                endIds: [placeholderId],
                endY: startY + nodeSize.placeholder.height,
            };
        }

        const branchLayout = layoutList(branchNodes, centerX, startY, branchKey);
        branchLayout.firstIds.forEach((firstId) => {
            connect(parent.id, firstId, parent.id, {
                branch: branchKey,
                label: branchLabel(branchKey),
            });
        });

        return branchLayout;
    }

    layoutList(state.nodes, rootCenterX, topPadding);

    return { nodes, edges };
}

function edgeMidpoint(edge: LayoutEdge, layoutMap: Map<string, LayoutNode>) {
    const source = layoutMap.get(edge.source);
    const target = layoutMap.get(edge.target);

    if (!source || !target) {
        return { x: 0, y: 0 };
    }

    const points = [
        { x: source.x + source.width / 2, y: source.y + source.height },
        ...(edge.vertices || []),
        { x: target.x + target.width / 2, y: target.y },
    ];

    let total = 0;
    for (let i = 1; i < points.length; i += 1) {
        total += Math.hypot(points[i].x - points[i - 1].x, points[i].y - points[i - 1].y);
    }

    let walked = 0;
    const half = total / 2;
    for (let i = 1; i < points.length; i += 1) {
        const previous = points[i - 1];
        const current = points[i];
        const segment = Math.hypot(current.x - previous.x, current.y - previous.y);

        if (walked + segment >= half) {
            const ratio = segment === 0 ? 0 : (half - walked) / segment;
            return {
                x: previous.x + (current.x - previous.x) * ratio,
                y: previous.y + (current.y - previous.y) * ratio,
            };
        }
        walked += segment;
    }

    return points[points.length - 1];
}

function graphPointToContainer(graph: Graph, container: HTMLElement, point: { x: number; y: number }) {
    const graphWithCoords = graph as Graph & {
        localToClient?: (x: number, y: number) => { x: number; y: number };
        localToPage?: (x: number, y: number) => { x: number; y: number };
    };
    const clientPoint = graphWithCoords.localToClient?.(point.x, point.y)
        || graphWithCoords.localToPage?.(point.x, point.y)
        || point;
    const rect = container.getBoundingClientRect();

    return {
        x: clientPoint.x - rect.left,
        y: clientPoint.y - rect.top,
    };
}

export const WorkflowGraph = forwardRef<WorkflowGraphHandle, WorkflowGraphProps>(function WorkflowGraph(props, ref) {
    const {
        defaultValue,
        onAddStep,
        onChange,
        onDeleteNode,
        onEditNode,
        onSelectNode,
        selectedNodeId: controlledSelectedNodeId,
        value,
    } = props;
    const containerRef = useRef<HTMLDivElement | null>(null);
    const graphRef = useRef<Graph | null>(null);
    const [internalState, setInternalState] = useState<WorkflowState>(() => defaultValue || { nodes: [] });
    const [internalSelectedId, setInternalSelectedId] = useState("");
    const [edgeButtons, setEdgeButtons] = useState<EdgeButton[]>([]);
    const layoutRef = useRef<LayoutResult>({ nodes: [], edges: [] });
    const layoutingRef = useRef(false);
    const previousLayoutNodeCountRef = useRef(0);
    const selectedNodeId = controlledSelectedNodeId ?? internalSelectedId;
    const workflow = value || internalState;
    const workflowRef = useRef(workflow);
    const onSelectNodeRef = useRef(onSelectNode);

    const layout = useMemo(
        () => buildLayout(workflow, selectedNodeId, { onEditNode, onDeleteNode }),
        [workflow, selectedNodeId, onEditNode, onDeleteNode],
    );

    useEffect(() => {
        workflowRef.current = workflow;
    }, [workflow]);

    useEffect(() => {
        onSelectNodeRef.current = onSelectNode;
    }, [onSelectNode]);

    const setWorkflow = useCallback((nextState: WorkflowState) => {
        const renumbered = renumberSteps(nextState);
        if (!value) {
            setInternalState(renumbered);
        }
        onChange?.(renumbered);
    }, [onChange, value]);

    const syncEdgeButtons = useCallback(() => {
        const graph = graphRef.current;
        const container = containerRef.current;
        if (!graph || !container) {
            return;
        }

        const layoutMap = new Map(layoutRef.current.nodes.map((node) => [node.node.id, node]));
        const nextButtons = layoutRef.current.edges
            .filter((edge) => edge.addButton !== false)
            .map((edge) => {
                const point = graphPointToContainer(graph, container, edgeMidpoint(edge, layoutMap));
                return {
                    id: edge.id,
                    sourceNodeId: edge.sourceNodeId,
                    branch: edge.branch,
                    x: point.x,
                    y: point.y,
                };
            });

        setEdgeButtons(nextButtons);
    }, []);

    const selectNode = useCallback((nodeId: string) => {
        const node = findWorkflowNode(workflowRef.current.nodes, nodeId);
        if (!node || node.type === "placeholder") {
            return;
        }
        setInternalSelectedId(nodeId);
        onSelectNodeRef.current?.(node);
    }, []);

    const addNode = useCallback((type: string, data: AddNodeInput = {}) => {
        const next = cloneWorkflow(workflow);
        const nextNode = createWorkflowNode(type, data, actionCount(next.nodes) + 1);

        if (data.branch && data.afterNodeId) {
            const parent = findWorkflowNode(next.nodes, data.afterNodeId);
            if (parent?.type === "approval") {
                parent.branches ||= { approved: [], rejected: [] };
                parent.branches[data.branch].push(nextNode);
            } else {
                insertAfter(next.nodes, data.afterNodeId, nextNode);
            }
        } else if (data.afterNodeId) {
            insertAfter(next.nodes, data.afterNodeId, nextNode);
        } else {
            next.nodes.push(nextNode);
        }

        setWorkflow(next);
        setInternalSelectedId(nextNode.id);
        onSelectNode?.(nextNode);
        return nextNode;
    }, [onSelectNode, setWorkflow, workflow]);

    const removeNode = useCallback((id: string) => {
        const node = findWorkflowNode(workflow.nodes, id);
        if (!node || node.type === "trigger") {
            return;
        }

        const next = cloneWorkflow(workflow);
        removeNodeById(next.nodes, id);
        setWorkflow(next);
        setInternalSelectedId("");
        onSelectNode?.(null);
    }, [onSelectNode, setWorkflow, workflow]);

    const moveNode = useCallback((id: string, position: { x: number; y: number }) => {
        const next = cloneWorkflow(workflow);
        const list = findSiblingList(next.nodes, id);
        if (!list || list.length < 2) {
            return;
        }

        const index = list.findIndex((node) => node.id === id);
        if (index < 0 || list[index].type === "trigger") {
            return;
        }

        const [node] = list.splice(index, 1);
        const layoutMap = new Map(layoutRef.current.nodes.map((layoutNode) => [layoutNode.node.id, layoutNode]));
        const nextIndex = list.findIndex((sibling) => {
            const layoutNode = layoutMap.get(sibling.id);
            return layoutNode ? position.y < layoutNode.y + layoutNode.height / 2 : false;
        });

        list.splice(nextIndex >= 0 ? nextIndex : list.length, 0, node);
        setWorkflow(next);
    }, [setWorkflow, workflow]);
    const moveNodeRef = useRef(moveNode);

    useEffect(() => {
        moveNodeRef.current = moveNode;
    }, [moveNode]);

    useImperativeHandle(ref, () => ({
        addNode,
        removeNode,
        moveNode,
        getGraph: () => graphRef.current,
    }), [addNode, moveNode, removeNode]);

    useEffect(() => {
        const container = containerRef.current;
        if (!container) {
            return;
        }

        registerWorkflowNodes();

        const graph = new Graph({
            container,
            autoResize: true,
            connecting: false as never,
            grid: {
                visible: true,
                type: "dot",
                args: {
                    color: "#d4dce8",
                    thickness: 1,
                },
            },
            background: {
                color: "#f8fafc",
            },
            panning: {
                enabled: true,
                eventTypes: ["rightMouseDown", "mouseWheel"],
            },
            mousewheel: {
                enabled: true,
                modifiers: ["ctrl", "meta"],
                minScale: 0.65,
                maxScale: 1.4,
            },
            interacting(cellView) {
                const data = cellView.cell.getData<WorkflowNodeRenderData>();
                return data?.type !== "placeholder" && data?.type !== "trigger";
            },
        });

        graphRef.current = graph;

        graph.on("node:click", ({ node }) => {
            selectNode(String(node.id));
        });
        graph.on("blank:click", () => {
            setInternalSelectedId("");
            onSelectNodeRef.current?.(null);
        });
        graph.on("node:moved", ({ node }) => {
            if (layoutingRef.current) {
                return;
            }
            const position = node.position();
            moveNodeRef.current(String(node.id), position);
        });
        graph.on("scale", syncEdgeButtons);
        graph.on("translate", syncEdgeButtons);
        graph.on("resize", syncEdgeButtons);

        return () => {
            graph.dispose();
            graphRef.current = null;
        };
    }, [selectNode, syncEdgeButtons]);

    useEffect(() => {
        const graph = graphRef.current;
        if (!graph) {
            return;
        }

        layoutRef.current = layout;
        layoutingRef.current = true;
        const graphWithBatching = graph as Graph & { freeze?: () => void; unfreeze?: () => void };
        graphWithBatching.freeze?.();
        graph.clearCells();

        layout.nodes.forEach((layoutNode) => {
            graph.addNode({
                id: layoutNode.node.id,
                shape: WORKFLOW_NODE_SHAPES[layoutNode.node.type],
                x: layoutNode.x,
                y: layoutNode.y,
                width: layoutNode.width,
                height: layoutNode.height,
                data: layoutNode.node,
                zIndex: 2,
            });
        });

        layout.edges.forEach((edge) => {
            graph.addEdge({
                id: edge.id,
                source: edge.source,
                target: edge.target,
                vertices: edge.vertices,
                connector: {
                    name: "rounded",
                    args: { radius: 10 },
                },
                attrs: {
                    line: {
                        stroke: edge.branch === "approved"
                            ? "#22c55e"
                            : edge.branch === "rejected"
                            ? "#ef4444"
                            : "#94a3b8",
                        strokeWidth: 2,
                        targetMarker: null,
                    },
                },
                labels: edge.label
                    ? [{
                        attrs: {
                            label: {
                                text: edge.label,
                                fill: edge.branch === "approved" ? "#15803d" : "#be123c",
                                fontSize: 11,
                                fontWeight: 700,
                            },
                            body: {
                                fill: "#fff",
                                stroke: edge.branch === "approved" ? "#bbf7d0" : "#fecdd3",
                                strokeWidth: 1,
                                rx: 8,
                                ry: 8,
                            },
                        },
                        position: 0.32,
                    }]
                    : undefined,
                data: {
                    sourceNodeId: edge.sourceNodeId,
                    branch: edge.branch,
                },
                zIndex: 1,
            });
        });

        graphWithBatching.unfreeze?.();
        layoutingRef.current = false;
        requestAnimationFrame(() => {
            syncEdgeButtons();

            const graphWithScroll = graph as Graph & {
                centerContent?: () => void;
                scrollToContent?: () => void;
            };
            if (layout.nodes.length > previousLayoutNodeCountRef.current) {
                graphWithScroll.scrollToContent?.() ?? graphWithScroll.centerContent?.();
            }
            previousLayoutNodeCountRef.current = layout.nodes.length;
        });
    }, [layout, syncEdgeButtons]);

    const handleDrop = useCallback((event: DragEvent<HTMLDivElement>) => {
        event.preventDefault();
        const raw = event.dataTransfer.getData("application/x-workflow-node");
        if (!raw) {
            return;
        }

        const payload = JSON.parse(raw) as { type: string; title?: string; subtitle?: string; icon?: string };
        const container = containerRef.current;
        if (!container) {
            addNode(payload.type, payload);
            return;
        }

        const rect = container.getBoundingClientRect();
        const localY = event.clientY - rect.top;
        const nearest = layoutRef.current.nodes
            .filter((layoutNode) => layoutNode.node.type !== "placeholder")
            .sort((a, b) => Math.abs(localY - (a.y + a.height / 2)) - Math.abs(localY - (b.y + b.height / 2)))[0];

        addNode(payload.type, {
            ...payload,
            afterNodeId: nearest?.node.id,
        });
    }, [addNode]);

    return (
        <div
            className="relative h-full w-full overflow-hidden bg-slate-50"
            onDragOver={(event) => event.preventDefault()}
            onDrop={handleDrop}
        >
            <div ref={containerRef} className="h-full w-full" />
            <div className="pointer-events-none absolute inset-0">
                {edgeButtons.map((button) => (
                    <button
                        key={button.id}
                        type="button"
                        aria-label="Add workflow step"
                        className="pointer-events-auto absolute grid h-7 w-7 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border border-blue-200 bg-white text-blue-600 shadow-sm transition hover:border-blue-500 hover:bg-blue-50"
                        style={{ left: button.x, top: button.y }}
                        onClick={() => onAddStep?.(button.sourceNodeId, button.branch)}
                    >
                        <i className="ri-add-line" aria-hidden />
                    </button>
                ))}
            </div>
        </div>
    );
});
