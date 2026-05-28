import type { ActionDefinition } from "./types";

export const workflowActions: ActionDefinition[] = [
    { type: "condition", title: "Condition", subtitle: "Branch when rules match", icon: "ti-git-merge" },
    { type: "code", title: "Code", subtitle: "Run custom logic", icon: "ti-code" },
    { type: "http", title: "HTTP request", subtitle: "Call an external URL", icon: "ti-world" },
    { type: "mail.send", title: "Send mail", subtitle: "Send an email message", icon: "ti-mail" },
    { type: "record.create", title: "Create record", subtitle: "Insert a PocketBase record", icon: "ti-plus" },
    { type: "record.update", title: "Update record", subtitle: "Modify a PocketBase record", icon: "ti-edit" },
    { type: "record.delete", title: "Delete record", subtitle: "Remove a PocketBase record", icon: "ti-trash" },
    { type: "capability", title: "Capability", subtitle: "Run a configured capability", icon: "ti-puzzle" },
    { type: "wait.delay", title: "Wait delay", subtitle: "Pause for a duration", icon: "ti-clock" },
    { type: "wait.webhook", title: "Wait webhook", subtitle: "Resume from webhook", icon: "ti-webhook" },
    { type: "wait.event", title: "Wait event", subtitle: "Resume from event", icon: "ti-radar" },
    {
        type: "wait.approval",
        title: "Wait approval",
        subtitle: "Split into approved and rejected paths",
        icon: "ti-user-check",
        nodeType: "approval",
    },
    { type: "ai.generate", title: "AI generate", subtitle: "Generate content", icon: "ti-sparkles" },
    { type: "ai.extract", title: "AI extract", subtitle: "Extract structured data", icon: "ti-scan" },
    { type: "ai.classify", title: "AI classify", subtitle: "Classify content", icon: "ti-category" },
];

export interface SidebarProps {
    actions?: ActionDefinition[];
    onAddAction?: (action: ActionDefinition) => void;
}

export function Sidebar(props: SidebarProps) {
    const actions = props.actions || workflowActions;

    return (
        <aside className="flex h-full w-80 shrink-0 flex-col border-r border-slate-200 bg-white">
            <div className="border-b border-slate-200 px-4 py-3">
                <div className="text-sm font-semibold text-slate-950">Actions</div>
            </div>
            <div className="flex-1 space-y-2 overflow-y-auto p-3">
                {actions.map((action) => (
                    <button
                        key={action.type}
                        type="button"
                        draggable
                        className="flex w-full cursor-grab items-center gap-3 rounded-lg border border-slate-200 bg-white p-3 text-left shadow-sm transition hover:border-blue-300 hover:bg-blue-50 active:cursor-grabbing"
                        onClick={() => props.onAddAction?.(action)}
                        onDragStart={(event) => {
                            event.dataTransfer.effectAllowed = "copy";
                            event.dataTransfer.setData("application/x-workflow-node", JSON.stringify(action));
                        }}
                    >
                        <span className="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-slate-100 text-lg text-slate-700">
                            <i className={action.icon} aria-hidden />
                        </span>
                        <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-semibold text-slate-950">{action.title}</span>
                            <span className="block truncate text-xs text-slate-500">{action.subtitle}</span>
                        </span>
                    </button>
                ))}
            </div>
        </aside>
    );
}
