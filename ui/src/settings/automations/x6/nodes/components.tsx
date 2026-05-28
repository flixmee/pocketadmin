import type { CSSProperties, MouseEvent, ReactNode } from "react";
import type { BranchKey, WorkflowNodeRenderData } from "../types";

const baseCard: CSSProperties = {
    width: "100%",
    height: "100%",
    boxSizing: "border-box",
    borderRadius: 8,
    background: "#fff",
    border: "1px solid #d8dee9",
    boxShadow: "0 8px 24px rgba(24, 39, 75, 0.08)",
    color: "#172033",
    fontFamily: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif",
};

const row: CSSProperties = {
    display: "flex",
    alignItems: "center",
    minWidth: 0,
};

const iconBox: CSSProperties = {
    display: "grid",
    placeItems: "center",
    width: 34,
    height: 34,
    flex: "0 0 34px",
    borderRadius: 8,
    background: "#eef6ff",
    color: "#2563eb",
    fontSize: 18,
};

const titleStyle: CSSProperties = {
    overflow: "hidden",
    textOverflow: "ellipsis",
    whiteSpace: "nowrap",
    fontSize: 13,
    fontWeight: 700,
    lineHeight: "18px",
};

const subtitleStyle: CSSProperties = {
    overflow: "hidden",
    textOverflow: "ellipsis",
    whiteSpace: "nowrap",
    color: "#64748b",
    fontSize: 12,
    lineHeight: "16px",
};

function stopCanvasEvent(e: MouseEvent) {
    e.stopPropagation();
}

function Badge(props: { children: ReactNode; tone?: "blue" | "green" | "red" | "gray" }) {
    const colors = {
        blue: ["#eff6ff", "#1d4ed8"],
        green: ["#ecfdf3", "#15803d"],
        red: ["#fff1f2", "#be123c"],
        gray: ["#f1f5f9", "#475569"],
    } as const;
    const [bg, fg] = colors[props.tone || "gray"];

    return (
        <span
            style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 4,
                borderRadius: 999,
                background: bg,
                color: fg,
                fontSize: 11,
                fontWeight: 700,
                lineHeight: "18px",
                padding: "0 8px",
                whiteSpace: "nowrap",
            }}
        >
            {props.children}
        </span>
    );
}

function ContextActions(
    props: { nodeId: string; onEdit?: (nodeId: string) => void; onDelete?: (nodeId: string) => void },
) {
    return (
        <div
            onMouseDown={stopCanvasEvent}
            onClick={stopCanvasEvent}
            style={{
                position: "absolute",
                right: 8,
                top: -34,
                display: "flex",
                gap: 6,
                padding: 4,
                borderRadius: 8,
                background: "#fff",
                border: "1px solid #d8dee9",
                boxShadow: "0 8px 24px rgba(24, 39, 75, 0.12)",
            }}
        >
            <button
                aria-label="Edit step"
                type="button"
                onClick={() => props.onEdit?.(props.nodeId)}
                style={actionButtonStyle}
            >
                <i className="ri-pencil-line" aria-hidden />
            </button>
            <button
                aria-label="Delete step"
                type="button"
                onClick={() => props.onDelete?.(props.nodeId)}
                style={{ ...actionButtonStyle, color: "#be123c" }}
            >
                <i className="ri-delete-bin-line" aria-hidden />
            </button>
        </div>
    );
}

const actionButtonStyle: CSSProperties = {
    width: 26,
    height: 26,
    display: "grid",
    placeItems: "center",
    border: "0",
    borderRadius: 6,
    background: "#f8fafc",
    color: "#334155",
    cursor: "pointer",
    fontSize: 15,
};

function StepMeta(props: { node: WorkflowNodeRenderData }) {
    return (
        <div style={{ ...row, justifyContent: "space-between", gap: 8, marginTop: 10 }}>
            <Badge tone="blue">Step {props.node.stepNumber || "-"}</Badge>
            {props.node.isValid
                ? (
                    <Badge tone="green">
                        <i className="ri-check-line" aria-hidden /> Valid
                    </Badge>
                )
                : <Badge>Draft</Badge>}
        </div>
    );
}

function NodeBody(props: { node: WorkflowNodeRenderData }) {
    const { node } = props;

    return (
        <>
            <div style={{ ...row, gap: 10 }}>
                {node.type !== "trigger" && (
                    <div
                        title="Drag step"
                        style={{
                            color: "#94a3b8",
                            cursor: "grab",
                            fontSize: 18,
                            lineHeight: 1,
                            paddingRight: 2,
                        }}
                    >
                        &#10303;
                    </div>
                )}
                <div style={iconBox}>
                    <i className={node.icon} aria-hidden />
                </div>
                <div style={{ minWidth: 0, flex: 1 }}>
                    <div style={titleStyle}>{node.title}</div>
                    <div style={subtitleStyle}>{node.subtitle}</div>
                </div>
            </div>
            {node.type !== "trigger" && <StepMeta node={node} />}
        </>
    );
}

export function TriggerNodeView(props: { node: WorkflowNodeRenderData }) {
    return (
        <div
            style={{
                ...baseCard,
                display: "flex",
                alignItems: "center",
                padding: "14px 16px",
                borderColor: props.node.selected ? "#2563eb" : "#cbd5e1",
                boxShadow: props.node.selected
                    ? "0 0 0 3px rgba(37, 99, 235, 0.16), 0 8px 24px rgba(24, 39, 75, 0.08)"
                    : baseCard.boxShadow,
            }}
        >
            <NodeBody node={props.node} />
        </div>
    );
}

export function ActionNodeView(props: { node: WorkflowNodeRenderData }) {
    const { node } = props;

    return (
        <div
            style={{
                ...baseCard,
                position: "relative",
                padding: "14px",
                borderColor: node.selected ? "#2563eb" : "#d8dee9",
                boxShadow: node.selected
                    ? "0 0 0 3px rgba(37, 99, 235, 0.16), 0 8px 24px rgba(24, 39, 75, 0.1)"
                    : baseCard.boxShadow,
            }}
        >
            {node.selected && node.deletable && (
                <ContextActions nodeId={node.id} onEdit={node.onEdit} onDelete={node.onDelete} />
            )}
            <NodeBody node={node} />
        </div>
    );
}

export function ApprovalNodeView(props: { node: WorkflowNodeRenderData }) {
    const { node } = props;

    return (
        <div
            style={{
                ...baseCard,
                position: "relative",
                padding: "14px",
                borderColor: node.selected ? "#2563eb" : "#d8dee9",
                boxShadow: node.selected
                    ? "0 0 0 3px rgba(37, 99, 235, 0.16), 0 8px 24px rgba(24, 39, 75, 0.1)"
                    : baseCard.boxShadow,
            }}
        >
            {node.selected && node.deletable && (
                <ContextActions nodeId={node.id} onEdit={node.onEdit} onDelete={node.onDelete} />
            )}
            <NodeBody node={node} />
            <div style={{ ...row, gap: 8, marginTop: 10 }}>
                <BranchBadge branch="approved" />
                <BranchBadge branch="rejected" />
            </div>
        </div>
    );
}

export function PlaceholderNodeView(props: { node: WorkflowNodeRenderData }) {
    const tone = props.node.branch === "approved" ? "#16a34a" : "#dc2626";

    return (
        <div
            style={{
                ...baseCard,
                display: "grid",
                placeItems: "center",
                border: `1px dashed ${tone}`,
                background: props.node.branch === "approved" ? "#f0fdf4" : "#fff1f2",
                color: "#475569",
                boxShadow: "none",
                padding: "12px",
                textAlign: "center",
            }}
        >
            <div>
                <BranchBadge branch={props.node.branch || "approved"} />
                <div style={{ marginTop: 8, fontSize: 12, fontWeight: 700 }}>
                    {props.node.placeholderText || "No steps"}
                </div>
            </div>
        </div>
    );
}

function BranchBadge(props: { branch: BranchKey }) {
    return (
        <Badge tone={props.branch === "approved" ? "green" : "red"}>
            {props.branch === "approved" ? "APPROVED" : "REJECTED"}
        </Badge>
    );
}
