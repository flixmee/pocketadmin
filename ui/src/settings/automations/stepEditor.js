import { conditionStepForm } from "./conditionStepForm";
import { httpStepForm } from "./httpStepForm";
import { mailStepForm } from "./mailStepForm";
import { recordStepForm } from "./recordStepForm";
import { responseStepForm } from "./responseStepForm";

const stepTypeOptions = [
    { value: "condition", label: "Condition", icon: "ri-git-merge-line", category: "control" },
    { value: "code", label: "Code", icon: "ri-code-s-slash-line", category: "control" },
    { value: "http", label: "HTTP request", icon: "ri-global-line", category: "integration" },
    { value: "mail.send", label: "Send mail", icon: "ri-mail-send-line", category: "communication" },
    { value: "record.create", label: "Create record", icon: "ri-add-box-line", category: "record" },
    { value: "record.update", label: "Update record", icon: "ri-edit-2-line", category: "record" },
    { value: "record.delete", label: "Delete record", icon: "ri-delete-bin-7-line", category: "record" },
    {
        value: "response",
        label: "Webhook response",
        icon: "ri-reply-line",
        category: "webhook",
        triggerTypes: ["webhook"],
    },
    { value: "capability", label: "Capability", icon: "ri-puzzle-2-line", category: "capability" },
    { value: "wait.delay", label: "Wait delay", icon: "ri-timer-line", category: "wait" },
    { value: "wait.webhook", label: "Wait webhook", icon: "ri-webhook-line", category: "wait" },
    { value: "wait.event", label: "Wait event", icon: "ri-radar-line", category: "wait" },
    { value: "wait.approval", label: "Wait approval", icon: "ri-user-follow-line", category: "approval" },
    { value: "ai.extract", label: "AI extract", icon: "ri-sparkling-2-line", category: "ai" },
    { value: "ai.classify", label: "AI classify", icon: "ri-sparkling-line", category: "ai" },
    { value: "ai.generate", label: "AI generate", icon: "ri-magic-line", category: "ai" },
    { value: "ai.summarize", label: "AI summarize", icon: "ri-file-reduce-line", category: "ai" },
];

const valueTypeOptions = [
    { value: "text", label: "Text" },
    { value: "number", label: "Number" },
    { value: "boolean", label: "Boolean" },
    { value: "template", label: "Template" },
];

const schemaTypeOptions = [
    { value: "string", label: "Text" },
    { value: "number", label: "Number" },
    { value: "integer", label: "Integer" },
    { value: "boolean", label: "Boolean" },
    { value: "array", label: "List" },
    { value: "object", label: "Object" },
];

const waitDurationUnitOptions = [
    { value: "s", label: "Seconds" },
    { value: "m", label: "Minutes" },
    { value: "h", label: "Hours" },
];

export function stepEditor(propsArg = {}) {
    const props = store({
        steps: [],
        errors: null,
        triggerType: "",
        triggerCollectionRef: "",
        onchange: function(steps) {},
    });

    const watchers = app.utils.extendStore(props, propsArg);
    let activeStepModal = null;
    let stopPointerDrag = null;
    let suppressedClickStepId = "";
    let suppressedClickUntil = 0;
    let dragState = null;
    let suppressedPaletteActionUntil = 0;

    const data = store({
        expandedById: {},
        mode: "visual",
        selectedStepId: "",
        schemas: null,
        isLoadingSchemas: false,
        schemaError: "",
        capabilityQuery: "",
        capabilityCategory: "",
        drawerOpen: false,
        drawerActiveTab: "settings",
        dragStepId: "",
        dragActionType: "",
        dragInsertIndex: -1,
    });

    async function loadSchemas() {
        if (data.schemas || data.isLoadingSchemas) {
            return;
        }

        data.isLoadingSchemas = true;
        data.schemaError = "";

        try {
            data.schemas = await app.pb.send("/api/automations/schemas", {
                requestKey: "automationStepEditor.schemas",
            });
        } catch (err) {
            if (!err?.isAbort) {
                data.schemaError = err?.message || "Failed to load automation schemas.";
            }
        }

        data.isLoadingSchemas = false;
    }

    function setSteps(steps) {
        props.onchange?.(steps);
    }

    function addStep(type, insertIndex = -1) {
        const nextStep = createReactiveEditorStep(type);
        data.expandedById[nextStep.__id] = true;
        data.selectedStepId = nextStep.__id;
        data.drawerOpen = true;
        data.drawerActiveTab = "settings";

        const nextSteps = [...(props.steps || [])];
        if (insertIndex >= 0 && insertIndex <= nextSteps.length) {
            nextSteps.splice(insertIndex, 0, nextStep);
        } else {
            nextSteps.push(nextStep);
        }
        setSteps(nextSteps);
        openStepEditModal(nextStep);
    }

    function addCapabilityStep(capabilityKey, insertIndex = -1) {
        const nextStep = createReactiveEditorStep("capability");
        nextStep.capability = capabilityKey;
        data.expandedById[nextStep.__id] = true;
        data.selectedStepId = nextStep.__id;
        data.drawerOpen = true;
        data.drawerActiveTab = "settings";

        const nextSteps = [...(props.steps || [])];
        if (insertIndex >= 0 && insertIndex <= nextSteps.length) {
            nextSteps.splice(insertIndex, 0, nextStep);
        } else {
            nextSteps.push(nextStep);
        }
        setSteps(nextSteps);
        openStepEditModal(nextStep);
    }

    function removeStep(stepId) {
        const nextSteps = (props.steps || []).filter((step) => step.__id !== stepId);
        delete data.expandedById[stepId];
        if (data.selectedStepId === stepId) {
            data.selectedStepId = nextSteps[0]?.__id || "";
            data.drawerOpen = !!data.selectedStepId;
            closeDrawer();
        }
        setSteps(nextSteps);
    }

    function changeStepType(step, nextType) {
        resetStep(step, createEditorStep(nextType, { __id: step.__id }));
        data.expandedById[step.__id] = true;
    }

    function toggleExpanded(stepId) {
        data.expandedById[stepId] = !isExpanded(stepId);
    }

    function selectStep(stepId) {
        data.selectedStepId = stepId;
        data.expandedById[stepId] = true;
        data.drawerOpen = true;
        data.drawerActiveTab = "settings";
        openStepEditModal(stepId);
    }

    function closeDrawer() {
        data.drawerOpen = false;
        if (activeStepModal) {
            app.modals.close(activeStepModal, true);
            activeStepModal = null;
        }
    }

    function moveStep(stepId, insertIndex, animate = false) {
        const nextSteps = [...(props.steps || [])];
        const fromIndex = nextSteps.findIndex((step) => step.__id === stepId);
        if (fromIndex < 0) {
            return false;
        }

        const [step] = nextSteps.splice(fromIndex, 1);
        let toIndex = insertIndex;
        if (fromIndex < insertIndex) {
            toIndex -= 1;
        }
        toIndex = Math.max(0, Math.min(toIndex, nextSteps.length));
        if (toIndex === fromIndex) {
            return false;
        }

        const previousRects = animate ? snapshotStepNodeRects() : null;
        nextSteps.splice(toIndex, 0, step);
        data.selectedStepId = step.__id;
        setSteps(nextSteps);
        if (previousRects) {
            animateStepNodeLayout(previousRects, stepId);
        }
        return true;
    }

    function beginDrag(stepId) {
        data.dragStepId = stepId;
    }

    function endDrag() {
        data.dragStepId = "";
    }

    function beginNodePointerDrag(e, stepId) {
        if (e.button !== undefined && e.button !== 0) {
            return;
        }

        stopPointerDrag?.();

        const sourceEl = e.currentTarget;
        const canvasEl = sourceEl?.closest?.(".automation-builder-canvas");
        const sourceRect = sourceEl?.getBoundingClientRect?.();
        if (!sourceEl || !sourceRect) {
            return;
        }

        const pointerId = e.pointerId;
        const startX = e.clientX;
        const startY = e.clientY;
        let hasStarted = false;

        const start = () => {
            if (hasStarted) {
                return;
            }
            hasStarted = true;
            sourceEl.setPointerCapture?.(pointerId);
            dragState = createNodeDragState(sourceEl, canvasEl, stepId, startX, startY);
            beginDrag(stepId);
            document.body.classList.add("automation-builder-node-dragging");
            queueDragFrame();
        };

        const cleanup = () => {
            window.removeEventListener("pointermove", handlePointerMove);
            window.removeEventListener("pointerup", handlePointerUp);
            window.removeEventListener("pointercancel", handlePointerCancel);
            document.body.classList.remove("automation-builder-node-dragging");
            if (dragState?.frame) {
                cancelAnimationFrame(dragState.frame);
            }
            dragState?.overlay?.remove();
            dragState = null;
            stopPointerDrag = null;
        };

        const finish = (event, shouldMove) => {
            if (hasStarted && shouldMove && dragState) {
                dragState.clientX = event.clientX;
                dragState.clientY = event.clientY;
                runDragFrame(true);
            }
            cleanup();
            if (hasStarted) {
                event.preventDefault();
                suppressedClickStepId = stepId;
                suppressedClickUntil = Date.now() + 350;
            }
            endDrag();
        };

        const handlePointerMove = (event) => {
            if (pointerId !== undefined && event.pointerId !== pointerId) {
                return;
            }

            const deltaX = Math.abs(event.clientX - startX);
            const deltaY = Math.abs(event.clientY - startY);
            if (!hasStarted && deltaX < 4 && deltaY < 4) {
                return;
            }

            start();
            event.preventDefault();
            if (dragState) {
                dragState.clientX = event.clientX;
                dragState.clientY = event.clientY;
                queueDragFrame();
            }
        };

        const handlePointerUp = (event) => {
            if (pointerId !== undefined && event.pointerId !== pointerId) {
                return;
            }
            finish(event, true);
        };

        const handlePointerCancel = (event) => {
            if (pointerId !== undefined && event.pointerId !== pointerId) {
                return;
            }
            finish(event, false);
        };

        stopPointerDrag = () => finish(new Event("pointercancel"), false);

        window.addEventListener("pointermove", handlePointerMove, { passive: false });
        window.addEventListener("pointerup", handlePointerUp);
        window.addEventListener("pointercancel", handlePointerCancel);
    }

    function beginActionPointerDrag(e, option) {
        if (e.button !== undefined && e.button !== 0) {
            return;
        }

        stopPointerDrag?.();

        const sourceEl = e.currentTarget;
        const editorEl = sourceEl?.closest?.(".automation-step-editor");
        const canvasEl = editorEl?.querySelector?.(".automation-builder-canvas");
        const sourceRect = sourceEl?.getBoundingClientRect?.();
        if (!sourceEl || !canvasEl || !sourceRect) {
            return;
        }

        const pointerId = e.pointerId;
        const startX = e.clientX;
        const startY = e.clientY;
        let hasStarted = false;

        const start = () => {
            if (hasStarted) {
                return;
            }
            hasStarted = true;
            sourceEl.setPointerCapture?.(pointerId);
            dragState = createActionDragState(sourceEl, canvasEl, option, startX, startY);
            data.dragActionType = option.value;
            data.dragInsertIndex = -1;
            document.body.classList.add("automation-builder-node-dragging");
            queueDragFrame();
        };

        const cleanup = () => {
            window.removeEventListener("pointermove", handlePointerMove);
            window.removeEventListener("pointerup", handlePointerUp);
            window.removeEventListener("pointercancel", handlePointerCancel);
            document.body.classList.remove("automation-builder-node-dragging");
            if (dragState?.frame) {
                cancelAnimationFrame(dragState.frame);
            }
            dragState?.overlay?.remove();
            dragState = null;
            data.dragActionType = "";
            data.dragInsertIndex = -1;
            stopPointerDrag = null;
        };

        const finish = (event, shouldInsert) => {
            if (hasStarted && shouldInsert && dragState) {
                dragState.clientX = event.clientX;
                dragState.clientY = event.clientY;
                runDragFrame(true);
            }
            cleanup();
            if (hasStarted) {
                event.preventDefault();
                suppressedPaletteActionUntil = Date.now() + 350;
            }
        };

        const handlePointerMove = (event) => {
            if (pointerId !== undefined && event.pointerId !== pointerId) {
                return;
            }

            const deltaX = Math.abs(event.clientX - startX);
            const deltaY = Math.abs(event.clientY - startY);
            if (!hasStarted && deltaX < 4 && deltaY < 4) {
                return;
            }

            start();
            event.preventDefault();
            if (dragState) {
                dragState.clientX = event.clientX;
                dragState.clientY = event.clientY;
                queueDragFrame();
            }
        };

        const handlePointerUp = (event) => {
            if (pointerId !== undefined && event.pointerId !== pointerId) {
                return;
            }
            finish(event, true);
        };

        const handlePointerCancel = (event) => {
            if (pointerId !== undefined && event.pointerId !== pointerId) {
                return;
            }
            finish(event, false);
        };

        stopPointerDrag = () => finish(new Event("pointercancel"), false);

        window.addEventListener("pointermove", handlePointerMove, { passive: false });
        window.addEventListener("pointerup", handlePointerUp);
        window.addEventListener("pointercancel", handlePointerCancel);
    }

    function queueDragFrame() {
        if (!dragState || dragState.frame) {
            return;
        }

        dragState.frame = requestAnimationFrame(() => runDragFrame());
    }

    function runDragFrame(isFinal = false) {
        if (!dragState) {
            return;
        }

        dragState.frame = 0;
        updateDragOverlay(dragState);
        const scrolled = autoScrollForDrag(dragState);
        const insertIndex = resolvePointerDropIndex(dragState.clientY);
        const isActionDrag = dragState.kind === "action";
        const canInsertAction = isActionDrag
            && isPointerInsideElement(dragState.canvasEl, dragState.clientX, dragState.clientY);
        if (isActionDrag) {
            data.dragInsertIndex = canInsertAction ? insertIndex : -1;
            if (isFinal && canInsertAction) {
                addStep(dragState.type, insertIndex);
            }
        } else if (insertIndex !== dragState.lastInsertIndex || isFinal) {
            dragState.lastInsertIndex = insertIndex;
            moveStep(dragState.stepId, insertIndex, true);
        }
        if (scrolled) {
            queueDragFrame();
        }
    }

    function isClickSuppressed(stepId) {
        return suppressedClickStepId === stepId && Date.now() < suppressedClickUntil;
    }

    function isExpanded(stepId) {
        return data.expandedById[stepId] !== false;
    }

    function stepEditOptions() {
        return {
            get steps() {
                return props.steps || [];
            },
            get schemas() {
                return data.schemas;
            },
            get errors() {
                return props.errors;
            },
            get triggerType() {
                return props.triggerType;
            },
            get triggerCollectionRef() {
                return props.triggerCollectionRef;
            },
            get drawerActiveTab() {
                return data.drawerActiveTab;
            },
            setDrawerActiveTab: (tab) => (data.drawerActiveTab = tab),
            addStep,
            removeStep,
            changeStepType,
            closeDrawer,
            onafterclose: (el) => {
                if (activeStepModal === el) {
                    activeStepModal = null;
                }
                data.drawerOpen = false;
                el?.remove();
            },
        };
    }

    function openStepEditModal(stepOrId) {
        const step = typeof stepOrId === "string"
            ? (props.steps || []).find((item) => item.__id === stepOrId)
            : stepOrId;
        if (!step) {
            return;
        }

        if (activeStepModal) {
            app.modals.close(activeStepModal, true);
            activeStepModal = null;
        }

        data.drawerOpen = true;
        const modal = renderStepEditModal(step, stepEditOptions());
        activeStepModal = modal;
        document.body.appendChild(modal);
        app.modals.open(modal);
    }

    return t.div(
        {
            pbEvent: "automationStepEditor",
            className: "automation-step-editor automation-visual-builder",
            onmount: () => loadSchemas(),
            onunmount: () => {
                if (activeStepModal) {
                    app.modals.close(activeStepModal, true);
                    activeStepModal = null;
                }
                stopPointerDrag?.();
                watchers.forEach((w) => w?.unwatch());
            },
        },
        () => {
            const stepsError = extractErrorMessage(resolveStepListError(props.errors));
            if (!stepsError) {
                return null;
            }

            return t.div(
                { className: "alert danger m-b-sm" },
                t.div({ className: "content" }, stepsError),
            );
        },
        t.div(
            { className: "automation-builder-toolbar" },
            t.div(
                { className: "automation-mode-switcher" },
                t.button(
                    {
                        type: "button",
                        className: () => `automation-mode-btn ${data.mode === "visual" ? "active" : ""}`,
                        onclick: () => (data.mode = "visual"),
                    },
                    t.span({ className: "txt" }, "Visual builder"),
                ),
                t.button(
                    {
                        type: "button",
                        className: () => `automation-mode-btn ${data.mode === "structured" ? "active" : ""}`,
                        onclick: () => (data.mode = "structured"),
                    },
                    t.span({ className: "txt" }, "Structured editor"),
                ),
            ),
            t.div(
                { className: "txt-sm txt-hint" },
                () => {
                    const limit = data.schemas?.limits?.maxSteps;
                    return limit
                        ? `${(props.steps || []).length}/${limit} steps`
                        : `${(props.steps || []).length} steps`;
                },
            ),
        ),
        () =>
            data.mode === "visual"
                ? renderVisualBuilder({
                    steps: props.steps || [],
                    schemas: data.schemas,
                    isLoadingSchemas: data.isLoadingSchemas,
                    schemaError: data.schemaError,
                    selectedStepId: data.selectedStepId,
                    triggerType: props.triggerType,
                    capabilityQuery: data.capabilityQuery,
                    capabilityCategory: data.capabilityCategory,
                    setCapabilityQuery: (value) => (data.capabilityQuery = value),
                    setCapabilityCategory: (value) => (data.capabilityCategory = value),
                    drawerOpen: data.drawerOpen,
                    drawerActiveTab: data.drawerActiveTab,
                    setDrawerActiveTab: (tab) => (data.drawerActiveTab = tab),
                    dragStepId: data.dragStepId,
                    selectStep,
                    closeDrawer,
                    addStep,
                    addCapabilityStep,
                    removeStep,
                    changeStepType,
                    beginDrag,
                    endDrag,
                    beginNodePointerDrag,
                    beginActionPointerDrag,
                    isClickSuppressed,
                    isPaletteActionClickSuppressed: () => Date.now() < suppressedPaletteActionUntil,
                    moveStep,
                    dragActionType: data.dragActionType,
                    dragInsertIndex: data.dragInsertIndex,
                    errors: props.errors,
                    triggerCollectionRef: props.triggerCollectionRef,
                })
                : null,
        () =>
            data.mode === "structured"
                ? renderStructuredStepList({
                    steps: props.steps || [],
                    errors: props.errors,
                    triggerType: props.triggerType,
                    triggerCollectionRef: props.triggerCollectionRef,
                    setSteps,
                    addStep,
                    removeStep,
                    selectStep,
                    changeStepType,
                    toggleExpanded,
                    isExpanded,
                    selectedStepId: data.selectedStepId,
                    schemas: data.schemas,
                })
                : null,
    );
}

export function normalizeAutomationEditorSteps(steps) {
    if (typeof steps === "string" && steps.trim()) {
        try {
            steps = JSON.parse(steps);
        } catch (_) {
            steps = [];
        }
    }

    if (!Array.isArray(steps)) {
        return [];
    }

    return steps.map((step) => createReactiveEditorStep(step?.type, step));
}

export function buildAutomationStepsPayload(steps) {
    return (steps || []).map((step, index) => buildStepPayload(step, index));
}

function renderStepForm(step, error, context = {}) {
    switch (step.type) {
        case "condition":
            return conditionStepForm({ step, error, ...context });
        case "code":
            return codeStepForm({ step, context });
        case "http":
            return httpStepForm({ step, error, ...context });
        case "mail.send":
            return mailStepForm({
                step,
                error,
                triggerType: () => context.triggerType,
                triggerCollectionRef: () => context.triggerCollectionRef,
            });
        case "record.create":
        case "record.update":
        case "record.delete":
            return recordStepForm({ step, error, ...context });
        case "response":
            return responseStepForm({ step, error, ...context });
        case "capability":
        case "wait.delay":
        case "wait.webhook":
        case "wait.event":
        case "wait.approval":
        case "ai.extract":
        case "ai.classify":
        case "ai.generate":
        case "ai.summarize":
            return genericJSONStepForm({ step, error, context });
        default:
            return t.div({ className: "txt-sm txt-danger" }, `Unsupported step type "${step.type}".`);
    }
}

function renderStructuredStepList(options) {
    return app.components.sortable({
        className: "list automation-steps-list",
        handle: ".sort-handle",
        data: () => options.steps,
        onchange: (sortedSteps) => options.setSteps(sortedSteps),
        before: () => {
            if (options.steps?.length) {
                return null;
            }

            return t.div(
                { className: "list-item" },
                t.div(
                    { className: "content block txt-hint" },
                    "No steps added yet. Add a condition, HTTP request, mail step, or record action below.",
                ),
            );
        },
        dataItem: (step, index) => {
            const stepError = resolveStepError(options.errors, index);

            return t.div(
                {
                    rid: step.__id,
                    className: () =>
                        `list-item automation-step-item ${options.selectedStepId === step.__id ? "selected" : ""}`,
                },
                t.div(
                    { className: "content block" },
                    t.div(
                        { className: "flex gap-10 flex-wrap m-b-sm" },
                        t.span(
                            {
                                className: "label handle sort-handle",
                                title: "Reorder step",
                            },
                            t.i({ className: "ri-draggable", ariaHidden: true }),
                            t.span({ className: "txt" }, () => `Step ${index + 1}`),
                        ),
                        t.span({ className: "txt-bold" }, () => summarizeStep(step)),
                        () => renderValidationLabel(step),
                        t.div({ className: "m-l-auto" }),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm secondary transparent circle",
                                ariaLabel: app.attrs.tooltip("Focus in builder"),
                                onclick: () => options.selectStep(step.__id),
                            },
                            t.i({ className: "ri-focus-3-line", ariaHidden: true }),
                        ),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm secondary transparent circle",
                                ariaLabel: app.attrs.tooltip(options.isExpanded(step.__id) ? "Collapse" : "Expand"),
                                onclick: () => options.toggleExpanded(step.__id),
                            },
                            t.i({
                                className: () =>
                                    options.isExpanded(step.__id) ? "ri-arrow-up-s-line" : "ri-arrow-down-s-line",
                                ariaHidden: true,
                            }),
                        ),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm secondary transparent circle",
                                ariaLabel: app.attrs.tooltip("Remove"),
                                onclick: () => options.removeStep(step.__id),
                            },
                            t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                        ),
                    ),
                    renderStepTypeSelect(step, options),
                    app.components.slide(
                        () => options.isExpanded(step.__id),
                        t.div(
                            { className: "m-t-sm" },
                            () => renderStepValidation(step),
                            () =>
                                renderStepForm(step, stepError, {
                                    triggerType: options.triggerType,
                                    triggerCollectionRef: options.triggerCollectionRef,
                                    schemas: options.schemas,
                                }),
                        ),
                    ),
                    () => renderServerStepError(stepError),
                ),
            );
        },
        after: () => {
            return t.div(
                { className: "list-item block" },
                t.div(
                    { className: "flex gap-5 flex-wrap" },
                    () =>
                        t.div(
                            { className: "flex gap-5 flex-wrap" },
                            ...stepTypeAddOptions(options.triggerType).map((option) =>
                                t.button(
                                    {
                                        rid: option.value,
                                        type: "button",
                                        className: "btn sm secondary transparent",
                                        onclick: () => options.addStep(option.value),
                                    },
                                    t.i({ className: "ri-add-line", ariaHidden: true }),
                                    t.span({ className: "txt" }, option.label),
                                )
                            ),
                        ),
                ),
            );
        },
    });
}

function renderVisualBuilder(options) {
    let contextMenuIndex = null;
    const contextMenu = t.div(
        {
            className: "dropdown sm popover",
            popover: "auto",
            style:
                "margin: 0; position: fixed; inset: auto; background: var(--surfaceColor); border: 1px solid var(--surfaceAlt2Color); border-radius: var(--borderRadius); box-shadow: 0 4px 12px rgba(0,0,0,0.1); padding: 5px; z-index: 9999; min-width: 140px;",
        },
        t.button(
            {
                type: "button",
                className: "dropdown-item txt-danger",
                onclick: () => {
                    if (contextMenuIndex) {
                        options.removeStep(contextMenuIndex);
                    }
                    if (contextMenu.hidePopover) contextMenu.hidePopover();
                },
            },
            t.i({ className: "ri-delete-bin-line" }),
            t.span({ className: "txt" }, "Delete step"),
        ),
    );

    const selectedStep = options.steps.find((step) => step.__id === options.selectedStepId) || options.steps[0] || null;
    return t.div(
        { className: "automation-n8n-builder" },
        contextMenu,
        renderActionPalette(options),
        t.div(
            { className: "automation-builder-canvas" },
            renderTriggerNode(options),
            renderBuilderConnector({
                title: "Add step after trigger",
                onclick: () => options.addStep("condition", 0),
            }),
            () => {
                if (!options.steps.length) {
                    return t.div(
                        { className: "automation-builder-empty" },
                        () => options.dragInsertIndex === 0 ? renderActionDropMarker(options.dragActionType) : null,
                        t.i({ className: "ri-node-tree", ariaHidden: true }),
                        t.div({ className: "txt-bold" }, "Start with a step"),
                        t.div({ className: "txt-sm txt-hint" }, "Use the action palette to add workflow blocks."),
                    );
                }

                return t.div(
                    { className: "automation-builder-step-nodes" },
                    ...options.steps.flatMap((step, index) => {
                        const validation = clientValidateStep(step);
                        const children = [];
                        if (options.dragInsertIndex === index) {
                            children.push(renderActionDropMarker(options.dragActionType));
                        }

                        children.push(
                            t.div(
                                { className: "automation-builder-node-wrap" },
                                t.button(
                                    {
                                        rid: step.__id,
                                        type: "button",
                                        "html-data-automation-step-node": "true",
                                        "html-data-automation-step-id": step.__id,
                                        className: () =>
                                            `automation-builder-node ${
                                                selectedStep?.__id === step.__id ? "selected" : ""
                                            } ${isAIStep(step.type) ? "ai-step" : ""} ${
                                                validation.length ? "has-issues" : ""
                                            } ${options.dragStepId === step.__id ? "dragging" : ""}`,
                                        onpointerdown: (e) => options.beginNodePointerDrag(e, step.__id),
                                        oncontextmenu: (e) => {
                                            e.preventDefault();
                                            e.stopPropagation();
                                            contextMenuIndex = step.__id;
                                            contextMenu.style.left = `${e.clientX}px`;
                                            contextMenu.style.top = `${e.clientY}px`;
                                            if (contextMenu.showPopover) {
                                                setTimeout(() => {
                                                    try {
                                                        contextMenu.showPopover();
                                                    } catch (_) {}
                                                }, 100);
                                            }
                                        },
                                        onclick: (e) => {
                                            if (options.isClickSuppressed(step.__id)) {
                                                e.preventDefault();
                                                return;
                                            }
                                            options.selectStep(step.__id);
                                        },
                                    },
                                    t.span(
                                        {
                                            className: "automation-builder-drag-handle",
                                            title: "Drag to reorder",
                                        },
                                        "⠿",
                                    ),
                                    t.div(
                                        { className: "automation-builder-node-icon" },
                                        t.i({ className: stepTypeIcon(step.type), ariaHidden: true }),
                                    ),
                                    t.div(
                                        { className: "content block txt-left" },
                                        t.div(
                                            { className: "automation-node-title m-b-5" },
                                            () => stepTypeLabel(step.type),
                                        ),
                                        t.div(
                                            { className: "automation-node-desc txt-ellipsis" },
                                            () => summarizeStep(step),
                                        ),
                                    ),
                                    t.div(
                                        { className: "automation-node-metadata" },
                                        t.span({ className: "automation-step-label" }, `Step ${index + 1}`),
                                        renderStepStatusBadge(validation),
                                    ),
                                    t.span({ className: "automation-builder-port input-port" }),
                                    t.span({ className: "automation-builder-port output-port" }),
                                ),
                                renderBuilderConnector({
                                    title: "Add connected step",
                                    onclick: () => options.addStep("condition", index + 1),
                                }),
                            ),
                        );

                        if (index === options.steps.length - 1 && options.dragInsertIndex === index + 1) {
                            children.push(renderActionDropMarker(options.dragActionType));
                        }

                        return children;
                    }),
                );
            },
        ),
    );
}

function renderBuilderConnector(attrs) {
    return t.div(
        { className: "automation-builder-connector" },
        t.span({ className: "automation-builder-connector-line" }),
        t.button(
            {
                type: "button",
                className: "automation-builder-plus",
                title: attrs.title,
                onclick: attrs.onclick,
            },
            t.i({ className: "ri-add-line", ariaHidden: true }),
        ),
        t.span({ className: "automation-builder-connector-line" }),
    );
}

function renderStepStatusBadge(validation) {
    if (!validation.length) {
        return t.span(
            { className: "automation-valid-badge" },
            t.i({ className: "ri-check-line", ariaHidden: true }),
            "Valid",
        );
    }

    return t.span(
        { className: "automation-valid-badge has-issues" },
        t.i({ className: "ri-error-warning-line", ariaHidden: true }),
        `${validation.length} issue(s)`,
    );
}

function isAIStep(type) {
    return String(type || "").startsWith("ai.");
}

function renderActionPalette(options) {
    return t.aside(
        { className: "automation-builder-palette" },
        t.div({ className: "txt-bold m-b-xs" }, "Actions"),
        t.div({ className: "txt-sm txt-hint m-b-sm" }, "Drag nodes in the canvas to reconnect the ordered flow."),
        t.div(
            { className: "automation-builder-palette-actions" },
            () =>
                t.div(
                    { className: "automation-builder-palette-actions-inner" },
                    ...stepTypeAddOptions(options.triggerType).map((option) =>
                        t.button(
                            {
                                type: "button",
                                className: () =>
                                    `automation-builder-palette-action ${
                                        options.dragActionType === option.value ? "dragging" : ""
                                    }`,
                                onpointerdown: (e) => options.beginActionPointerDrag(e, option),
                                onclick: (e) => {
                                    if (options.isPaletteActionClickSuppressed()) {
                                        e.preventDefault();
                                        return;
                                    }
                                    options.addStep(option.value);
                                },
                            },
                            t.div(
                                { className: "automation-palette-icon-wrap" },
                                t.i({ className: option.icon || "ri-add-line", ariaHidden: true }),
                            ),
                            t.div(
                                { className: "content block txt-left" },
                                t.div({ className: "automation-node-title" }, option.label),
                                t.div({ className: "automation-node-meta" }, option.category || "step"),
                            ),
                        )
                    ),
                ),
        ),
    );
}

function renderTriggerNode(options) {
    return t.div(
        { className: "automation-builder-node trigger-node" },
        t.div({ className: "automation-builder-node-icon" }, t.i({ className: "ri-flashlight-line" })),
        t.div(
            { className: "content block" },
            t.div({ className: "automation-node-title m-b-5" }, "Trigger"),
            t.div({ className: "automation-node-desc" }, () => options.triggerType || "manual"),
        ),
        t.span({ className: "automation-builder-port output-port" }),
    );
}

function renderActionDropMarker(type) {
    return t.div(
        { className: "automation-builder-drop-marker" },
        t.div(
            { className: "automation-builder-drop-node" },
            t.div(
                { className: "automation-builder-node-icon" },
                t.i({ className: stepTypeIcon(type), ariaHidden: true }),
            ),
            t.div(
                { className: "content block txt-left" },
                t.div({ className: "automation-node-title" }, () => stepTypeLabel(type || "condition")),
                t.div({ className: "automation-node-meta" }, "Drop to add step"),
            ),
        ),
    );
}

function createNodeDragState(sourceEl, canvasEl, stepId, clientX, clientY) {
    const rect = sourceEl.getBoundingClientRect();
    const overlay = sourceEl.cloneNode(true);
    overlay.removeAttribute("rid");
    overlay.removeAttribute("id");
    overlay.classList.add("automation-builder-node-overlay");
    overlay.classList.remove("selected");
    overlay.style.width = `${rect.width}px`;
    overlay.style.height = `${rect.height}px`;
    overlay.style.left = "0px";
    overlay.style.top = "0px";
    overlay.style.transform = `translate3d(${rect.left}px, ${rect.top}px, 0) scale(1.03)`;
    document.body.appendChild(overlay);

    return {
        kind: "step",
        stepId,
        overlay,
        canvasEl,
        clientX,
        clientY,
        offsetX: clientX - rect.left,
        offsetY: clientY - rect.top,
        lastInsertIndex: -1,
        frame: 0,
    };
}

function createActionDragState(sourceEl, canvasEl, option, clientX, clientY) {
    const rect = sourceEl.getBoundingClientRect();
    const overlay = sourceEl.cloneNode(true);
    overlay.removeAttribute("rid");
    overlay.removeAttribute("id");
    overlay.classList.add("automation-builder-palette-action-overlay");
    overlay.classList.remove("dragging");
    overlay.style.width = `${rect.width}px`;
    overlay.style.height = `${rect.height}px`;
    overlay.style.left = "0px";
    overlay.style.top = "0px";
    overlay.style.transform = `translate3d(${rect.left}px, ${rect.top}px, 0) scale(1.03)`;
    document.body.appendChild(overlay);

    return {
        kind: "action",
        type: option.value,
        overlay,
        canvasEl,
        clientX,
        clientY,
        offsetX: clientX - rect.left,
        offsetY: clientY - rect.top,
        lastInsertIndex: -1,
        frame: 0,
    };
}

function updateDragOverlay(state) {
    if (!state?.overlay) {
        return;
    }

    const x = state.clientX - state.offsetX;
    const y = state.clientY - state.offsetY;
    state.overlay.style.transform = `translate3d(${x}px, ${y}px, 0) scale(1.035)`;
}

function autoScrollForDrag(state) {
    if (!state) {
        return false;
    }

    const edgeSize = 72;
    const maxScroll = 18;
    let didScroll = false;

    const scrollByEdge = (el, top, bottom, scrollFn) => {
        let delta = 0;
        if (state.clientY < top + edgeSize) {
            delta = -Math.round(maxScroll * (1 - Math.max(0, state.clientY - top) / edgeSize));
        } else if (state.clientY > bottom - edgeSize) {
            delta = Math.round(maxScroll * (1 - Math.max(0, bottom - state.clientY) / edgeSize));
        }

        if (delta) {
            scrollFn(delta);
            didScroll = true;
        }
    };

    if (state.canvasEl && state.canvasEl.scrollHeight > state.canvasEl.clientHeight) {
        const rect = state.canvasEl.getBoundingClientRect();
        scrollByEdge(state.canvasEl, rect.top, rect.bottom, (delta) => {
            state.canvasEl.scrollTop += delta;
        });
    }

    scrollByEdge(window, 0, window.innerHeight, (delta) => window.scrollBy(0, delta));

    return didScroll;
}

function snapshotStepNodeRects() {
    const rects = new Map();
    document
        .querySelectorAll(".automation-step-editor .automation-builder-node[data-automation-step-id]")
        .forEach((node) => {
            rects.set(node.dataset.automationStepId, node.getBoundingClientRect());
        });
    return rects;
}

function animateStepNodeLayout(previousRects, draggedStepId) {
    requestAnimationFrame(() => {
        document
            .querySelectorAll(".automation-step-editor .automation-builder-node[data-automation-step-id]")
            .forEach((node) => {
                const id = node.dataset.automationStepId;
                if (id === draggedStepId) {
                    return;
                }

                const previous = previousRects.get(id);
                if (!previous) {
                    return;
                }

                const current = node.getBoundingClientRect();
                const deltaX = previous.left - current.left;
                const deltaY = previous.top - current.top;
                if (!deltaX && !deltaY) {
                    return;
                }

                node.style.transition = "none";
                node.style.transform = `translate3d(${deltaX}px, ${deltaY}px, 0)`;
                void node.offsetHeight;
                requestAnimationFrame(() => {
                    node.style.transition = "";
                    node.style.transform = "";
                });
            });
    });
}

function resolvePointerDropIndex(clientY) {
    const nodes = Array.from(
        document.querySelectorAll(".automation-step-editor .automation-builder-node[data-automation-step-node='true']"),
    );

    if (!nodes.length) {
        return 0;
    }

    for (let i = 0; i < nodes.length; i++) {
        const rect = nodes[i].getBoundingClientRect();
        if (clientY < rect.top + rect.height / 2) {
            return i;
        }
    }

    return nodes.length;
}

function isPointerInsideElement(el, clientX, clientY) {
    if (!el) {
        return false;
    }

    const rect = el.getBoundingClientRect();
    return clientX >= rect.left && clientX <= rect.right && clientY >= rect.top && clientY <= rect.bottom;
}

function renderStepEditModal(step, options) {
    return t.div(
        {
            pbEvent: "automationStepEditModal",
            className: "modal popup lg automation-builder-drawer-shell automation-step-edit-modal",
            onafterclose: (el) => options.onafterclose?.(el),
        },
        t.header(
            { className: "modal-header isolated automation-builder-drawer-header" },
            t.div(
                { className: "automation-builder-node-icon" },
                t.i({ className: () => stepTypeIcon(step.type), ariaHidden: true }),
            ),
            t.div(
                { className: "content block" },
                t.div({ className: "txt-bold" }, () => stepTypeLabel(step.type)),
                t.div(
                    { className: "txt-sm txt-hint" },
                    () => {
                        const stepIndex = options.steps.findIndex((item) => item.__id === step.__id);
                        return stepIndex >= 0 ? `Step ${stepIndex + 1}` : "Step";
                    },
                ),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn sm secondary transparent circle",
                    ariaLabel: app.attrs.tooltip("Close"),
                    onclick: () => app.modals.close(),
                },
                t.i({ className: "ri-close-line", ariaHidden: true }),
            ),
        ),
        t.div(
            { className: "modal-content" },
            t.nav(
                { className: "tabs-header equal-width m-b-base" },
                t.button(
                    {
                        type: "button",
                        className: () => `tab-item ${options.drawerActiveTab === "settings" ? "active" : ""}`,
                        onclick: () => options.setDrawerActiveTab("settings"),
                    },
                    t.span({ className: "txt" }, "Settings"),
                ),
                t.button(
                    {
                        type: "button",
                        className: () => `tab-item ${options.drawerActiveTab === "mapping" ? "active" : ""}`,
                        onclick: () => options.setDrawerActiveTab("mapping"),
                    },
                    t.span({ className: "txt" }, "Data"),
                ),
                t.button(
                    {
                        type: "button",
                        className: () => `tab-item ${options.drawerActiveTab === "schema" ? "active" : ""}`,
                        onclick: () => options.setDrawerActiveTab("schema"),
                    },
                    t.span({ className: "txt" }, "Schema"),
                ),
            ),
            () => {
                const stepIndex = options.steps.findIndex((item) => item.__id === step.__id);
                const stepError = resolveStepError(options.errors, stepIndex);
                if (options.drawerActiveTab === "mapping") {
                    return renderDrawerMappingTab(options);
                }
                if (options.drawerActiveTab === "schema") {
                    return renderSchemaInspector(step, options.schemas, options);
                }

                return t.div(
                    { className: "automation-builder-drawer-form" },
                    renderStepTypeSelect(step, options),
                    () => renderStepValidation(step),
                    () => renderServerStepError(stepError),
                    () =>
                        renderStepForm(step, stepError, {
                            triggerType: options.triggerType,
                            triggerCollectionRef: options.triggerCollectionRef,
                            schemas: options.schemas,
                        }),
                );
            },
        ),
        t.footer(
            { className: "modal-footer automation-builder-drawer-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn secondary transparent m-r-auto",
                    onclick: () => {
                        const stepIndex = options.steps.findIndex((item) => item.__id === step.__id);
                        options.addStep("condition", stepIndex + 1);
                    },
                },
                t.i({ className: "ri-link", ariaHidden: true }),
                t.span({ className: "txt" }, "Connect next"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn secondary transparent txt-danger",
                    onclick: () => options.removeStep(step.__id),
                },
                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Remove"),
            ),
        ),
    );
}

function renderDrawerMappingTab(options) {
    return t.div(
        { className: "automation-builder-drawer-form" },
        t.div({ className: "txt-bold m-b-xs" }, "Template data"),
        t.div(
            { className: "txt-sm txt-hint m-b-sm" },
            "Copy a token and paste it into any text or JSON field in the Settings tab.",
        ),
        renderMappingPalette(options),
    );
}

function renderStepTypeSelect(step, options) {
    return t.div(
        { className: "grid" },
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${step.__id}_type` }, "Step type"),
                app.components.select({
                    id: `${step.__id}_type`,
                    value: () => step.type,
                    options: () => stepTypeSelectOptions(options.triggerType, step.type),
                    onchange: (selected) => {
                        const nextType = selected?.[0]?.value || "condition";
                        if (nextType !== step.type) {
                            options.changeStepType(step, nextType);
                        }
                    },
                }),
            ),
        ),
    );
}

function renderValidationLabel(step) {
    const validation = clientValidateStep(step);
    if (!validation.length) {
        return t.span({ className: "label success" }, "Valid");
    }

    return t.span({ className: "label warning" }, `${validation.length} issue(s)`);
}

function renderServerStepError(stepError) {
    const message = extractErrorMessage(stepError);
    if (!message) {
        return null;
    }

    return t.div(
        { className: "field-error txt-danger m-t-sm" },
        message,
    );
}

function renderMappingPalette(options) {
    return t.div(
        { className: "automation-builder-card" },
        t.div({ className: "txt-bold m-b-xs" }, "Data mapping"),
        t.div(
            { className: "txt-sm txt-hint m-b-sm" },
            "Copy a token, then paste it into any text or JSON field.",
        ),
        t.div(
            { className: "automation-token-groups" },
            () =>
                t.div(
                    { className: "automation-token-groups-inner" },
                    ...app.utils.automationMappingTokenGroups({
                        triggerType: () => options.triggerType,
                        triggerCollectionRef: () => options.triggerCollectionRef,
                        steps: () => options.steps,
                    })
                        .map((group) =>
                            t.div(
                                { className: "automation-token-group" },
                                t.div({ className: "txt-xs txt-hint" }, group.title),
                                t.div(
                                    { className: "flex gap-5 flex-wrap" },
                                    ...group.tokens.map((token) =>
                                        t.button(
                                            {
                                                type: "button",
                                                className: "label code-like",
                                                onclick: () => {
                                                    app.utils.copyToClipboard(token);
                                                    app.toasts.success("Mapping token copied.");
                                                },
                                            },
                                            token,
                                        )
                                    ),
                                ),
                            )
                        ),
                ),
        ),
    );
}

function renderCapabilityBrowser(options) {
    const capabilities = Object.values(options.schemas?.capabilities || {});
    const categories = [...new Set(capabilities.map((capability) => capability.category).filter(Boolean))].sort();

    return t.div(
        { className: "automation-builder-card" },
        t.div(
            { className: "flex gap-5 flex-wrap m-b-sm" },
            t.div({ className: "txt-bold" }, "Capability browser"),
            () => options.isLoadingSchemas ? t.span({ className: "label info" }, "Loading") : null,
        ),
        () => {
            if (options.schemaError) {
                return t.div({ className: "txt-sm txt-danger" }, options.schemaError);
            }

            if (!capabilities.length) {
                return t.div({ className: "txt-sm txt-hint" }, "No capability schemas are available.");
            }

            return t.div(
                { className: "automation-capability-browser-content" },
                t.div(
                    { className: "grid gap-sm" },
                    t.div(
                        { className: "col-md-7" },
                        t.div(
                            { className: "field" },
                            t.input({
                                type: "search",
                                placeholder: "Search capabilities",
                                value: () => options.capabilityQuery,
                                oninput: (e) => options.setCapabilityQuery(e.target.value),
                            }),
                        ),
                    ),
                    t.div(
                        { className: "col-md-5" },
                        app.components.select({
                            value: () => options.capabilityCategory,
                            options: [{ value: "", label: "All categories" }].concat(
                                categories.map((category) => ({ value: category, label: category })),
                            ),
                            onchange: (selected) => options.setCapabilityCategory(selected?.[0]?.value || ""),
                        }),
                    ),
                ),
                t.div(
                    { className: "automation-capability-list" },
                    () =>
                        t.div(
                            { className: "automation-capability-list-inner" },
                            ...filteredCapabilities(capabilities, options).map((capability) =>
                                t.button(
                                    {
                                        type: "button",
                                        className: "automation-capability-item",
                                        onclick: () => options.addCapabilityStep(capability.key),
                                    },
                                    t.i({ className: "ri-puzzle-2-line", ariaHidden: true }),
                                    t.div(
                                        { className: "content block txt-left" },
                                        t.div({ className: "txt-bold" }, capability.key),
                                        t.div(
                                            { className: "txt-sm txt-hint" },
                                            () =>
                                                `${capability.category || "capability"} • v${
                                                    capability.version || "1"
                                                }`,
                                        ),
                                    ),
                                    t.i({ className: "ri-add-line m-l-auto", ariaHidden: true }),
                                )
                            ),
                        ),
                ),
            );
        },
    );
}

function renderSchemaInspector(step, schemas, options) {
    return t.div(
        { className: "automation-builder-card" },
        t.div({ className: "txt-bold m-b-sm" }, "Inspector"),
        () => {
            if (!step) {
                return t.div({ className: "txt-sm txt-hint" }, "Select a step to inspect its schema and mappings.");
            }

            const schema = schemas?.steps?.[step.type] || null;
            const validation = clientValidateStep(step);
            return t.div(
                { className: "automation-builder-inspector-content" },
                t.div(
                    { className: "flex gap-5 flex-wrap m-b-sm" },
                    t.span({ className: "label" }, () => step.type),
                    t.span(
                        { className: () => `label ${validation.length ? "warning" : "success"}` },
                        () => validation.length ? `${validation.length} issue(s)` : "Valid",
                    ),
                ),
                t.div({ className: "txt-sm m-b-sm" }, () => summarizeStep(step)),
                renderStepValidation(step),
                () => {
                    if (!schema) {
                        return t.div({ className: "txt-sm txt-hint" }, "No schema metadata loaded for this step.");
                    }

                    return t.div(
                        { className: "automation-builder-schema-sections" },
                        renderSchemaProperties("Inputs", schema.inputSchema),
                        renderSchemaProperties("Outputs", schema.outputSchema),
                    );
                },
                t.div(
                    { className: "flex gap-5 flex-wrap m-t-sm" },
                    t.button(
                        {
                            type: "button",
                            className: "btn sm secondary transparent",
                            onclick: () => options.selectStep(step.__id),
                        },
                        t.i({ className: "ri-edit-2-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Edit below"),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: "btn sm secondary transparent",
                            onclick: () => options.removeStep(step.__id),
                        },
                        t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Remove"),
                    ),
                ),
            );
        },
    );
}

function renderSchemaProperties(title, schema) {
    const properties = schema?.properties || {};
    const keys = Object.keys(properties);
    if (!keys.length) {
        return null;
    }

    return t.div(
        { className: "m-t-sm" },
        t.div({ className: "txt-xs txt-hint m-b-xs" }, title),
        t.div(
            { className: "automation-schema-fields" },
            ...keys.map((key) =>
                t.span(
                    { className: "label" },
                    key,
                    t.span({ className: "txt-hint" }, `:${properties[key]?.type || "any"}`),
                )
            ),
        ),
    );
}

function renderStepValidation(step) {
    const messages = clientValidateStep(step);
    if (!messages.length) {
        return null;
    }

    return t.div(
        { className: "alert warning automation-validation m-b-sm" },
        t.div(
            { className: "content" },
            ...messages.map((message) => t.div(null, message)),
        ),
    );
}

function stepTypeAddOptions(triggerType) {
    return stepTypeOptions.filter((option) => !option.triggerTypes || option.triggerTypes.includes(triggerType));
}

function stepTypeSelectOptions(triggerType, selectedType) {
    const options = stepTypeAddOptions(triggerType);
    if (options.find((option) => option.value === selectedType)) {
        return options;
    }

    const selected = stepTypeOptions.find((option) => option.value === selectedType);
    return selected ? [selected, ...options] : options;
}

function createEditorStep(type, rawStep = {}) {
    const base = {
        __id: rawStep.__id || app.utils.randomString(),
        type: type || rawStep?.type || "condition",
    };

    switch (base.type) {
        case "condition":
            return {
                ...base,
                path: toString(rawStep.path || rawStep.field),
                op: toString(rawStep.op) || "exists",
                valueText: rawStep.value === undefined
                    ? toString(rawStep.valueText)
                    : stringifyLooseValue(rawStep.value),
                match: ["and", "or"].includes(rawStep.match) ? rawStep.match : "and",
                conditions: normalizeConditionRows(rawStep),
            };
        case "code":
            return {
                ...base,
                code: toString(rawStep.code) || "\n\nreturn {\n    value: record.id,\n};",
            };
        case "http":
            return {
                ...base,
                method: toString(rawStep.method) || "GET",
                url: toString(rawStep.url),
                headersText: stringifyJSONObject(rawStep.headers, "{}"),
                bodyText: stringifyLooseValue(rawStep.body),
                timeoutText: rawStep.timeout === undefined || rawStep.timeout === null ? "" : String(rawStep.timeout),
            };
        case "mail.send":
            return {
                ...base,
                toText: stringifyStringArray(rawStep.to),
                ccText: stringifyStringArray(rawStep.cc),
                bccText: stringifyStringArray(rawStep.bcc),
                subject: toString(rawStep.subject),
                text: toString(rawStep.text),
                html: toString(rawStep.html),
                attachments: normalizeStringArray(rawStep.attachments),
            };
        case "record.create":
            return {
                ...base,
                collection: toString(rawStep.collection),
                dataText: toString(rawStep.dataText) || stringifyJSONObject(rawStep.data, "{}"),
            };
        case "record.update":
            return {
                ...base,
                collection: toString(rawStep.collection),
                id: toString(rawStep.id),
                filter: toString(rawStep.filter),
                dataText: toString(rawStep.dataText) || stringifyJSONObject(rawStep.data, "{}"),
            };
        case "record.delete":
            return {
                ...base,
                collection: toString(rawStep.collection),
                id: toString(rawStep.id),
                filter: toString(rawStep.filter),
            };
        case "response":
            return {
                ...base,
                statusCodeText: rawStep.statusCodeText !== undefined && rawStep.statusCodeText !== null
                    ? String(rawStep.statusCodeText)
                    : rawStep.statusCode === undefined || rawStep.statusCode === null
                    ? "200"
                    : String(rawStep.statusCode),
                headersText: rawStep.headersText !== undefined
                    ? toString(rawStep.headersText)
                    : stringifyJSONObject(rawStep.headers, "{}"),
                bodyText: rawStep.bodyText !== undefined
                    ? toString(rawStep.bodyText)
                    : stringifyLooseValue(rawStep.body),
            };
        case "capability":
            return createCapabilityStep(base, rawStep);
        case "wait.delay":
            return createWaitDelayStep(base, rawStep);
        case "wait.webhook":
        case "wait.event":
            return createWaitKeyStep(base, rawStep);
        case "wait.approval":
            return createWaitApprovalStep(base, rawStep);
        case "ai.extract":
        case "ai.classify":
        case "ai.generate":
        case "ai.summarize":
            return createAIStep(base, rawStep);
        default:
            return createEditorStep("condition", { __id: base.__id });
    }
}

function createCapabilityStep(base, rawStep = {}) {
    return {
        ...base,
        capability: toString(rawStep.capability || rawStep.key),
        connectorRef: toString(rawStep.connectorRef),
        requiredScopes: normalizeStringArray(rawStep.requiredScopes),
        inputRows: objectToConfigRows(rawStep.input),
    };
}

function createWaitDelayStep(base, rawStep = {}) {
    const parsed = parseDurationParts(rawStep.duration);
    return {
        ...base,
        durationValue: parsed.value,
        durationUnit: parsed.unit,
    };
}

function createWaitKeyStep(base, rawStep = {}) {
    return {
        ...base,
        key: toString(rawStep.key || rawStep.event),
    };
}

function createWaitApprovalStep(base, rawStep = {}) {
    return {
        ...base,
        assignee: toString(rawStep.assignee),
        role: toString(rawStep.role),
        comment: toString(rawStep.comment),
    };
}

function createAIStep(base, rawStep = {}) {
    return {
        ...base,
        model: toString(rawStep.model),
        inputText: rawStep.inputText !== undefined ? toString(rawStep.inputText) : stringifyLooseValue(rawStep.input),
        labels: aiLabelRows(rawStep.labels),
        schemaRows: Array.isArray(rawStep.schemaRows)
            ? rawStep.schemaRows.map((row) => createSchemaRow(row))
            : schemaToRows(rawStep.schema),
    };
}

function aiLabelRows(labels) {
    if (Array.isArray(labels) && labels.some((label) => label && typeof label === "object")) {
        return labels
            .map((label) => toString(label?.value).trim())
            .filter(Boolean)
            .map((value) => ({
                __id: app.utils.randomString(),
                value,
            }));
    }

    return normalizeStringArray(labels).map((label) => ({
        __id: app.utils.randomString(),
        value: label,
    }));
}

function createReactiveEditorStep(type, rawStep = {}) {
    return store(createEditorStep(type, rawStep));
}

function buildStepPayload(step, index) {
    switch (step.type) {
        case "condition":
            return buildConditionPayload(step, index);
        case "code":
            return buildCodePayload(step, index);
        case "http":
            return buildHTTPPayload(step, index);
        case "mail.send":
            return buildMailPayload(step, index);
        case "record.create":
            return buildRecordCreatePayload(step, index);
        case "record.update":
            return buildRecordUpdatePayload(step, index);
        case "record.delete":
            return buildRecordDeletePayload(step, index);
        case "response":
            return buildResponsePayload(step, index);
        case "capability":
            return buildCapabilityPayload(step, index);
        case "wait.delay":
            return buildWaitDelayPayload(step, index);
        case "wait.webhook":
        case "wait.event":
            return buildWaitKeyPayload(step, index);
        case "wait.approval":
            return buildWaitApprovalPayload(step, index);
        case "ai.extract":
        case "ai.classify":
        case "ai.generate":
        case "ai.summarize":
            return buildAIPayload(step, index);
        default:
            throw new Error(`Step ${index + 1}: unsupported step type "${step.type}".`);
    }
}

function genericJSONStepForm({ step, context = {} }) {
    switch (step.type) {
        case "capability":
            return capabilityStepForm(step, context);
        case "code":
            return codeStepForm({ step, context });
        case "wait.delay":
            return waitDelayStepForm(step);
        case "wait.webhook":
        case "wait.event":
            return waitKeyStepForm(step, context);
        case "wait.approval":
            return waitApprovalStepForm(step, context);
        case "ai.extract":
        case "ai.classify":
        case "ai.generate":
        case "ai.summarize":
            return aiStepForm(step, context);
        default:
            return t.div({ className: "txt-sm txt-danger" }, `Unsupported step type "${step.type}".`);
    }
}

function codeStepForm({ step, context = {} }) {
    return t.div(
        { className: "automation-graphical-config" },
        t.div(
            { className: "field" },
            t.label({ htmlFor: `${step.__id}_code` }, "JavaScript code"),
            app.components.monacoEditor({
                id: `${step.__id}_code`,
                language: "javascript",
                placeholder: "return { total: record.amount * 1.1 };",
                value: () => step.code,
                autocomplete: () => [
                    { value: "record", label: "record" },
                    { value: "recordOriginal", label: "recordOriginal" },
                    { value: "$record", label: "$record" },
                    { value: "$recordOriginal", label: "$recordOriginal" },
                    { value: "trigger", label: "trigger" },
                    { value: "request", label: "request" },
                    { value: "i18n", label: "i18n" },
                    { value: "steps", label: "steps" },
                    { value: "prevStep", label: "prevStep" },
                    { value: "output", label: "output" },
                ],
                oninput: (value) => (step.code = value),
            }),
        ),
        t.div(
            { className: "txt-sm txt-hint" },
            "Return a JSON object. The result is available to later steps as ",
            t.code("prevStep.output"),
            " or ",
            t.code("steps[n].output"),
            ".",
        ),
    );
}

function capabilityStepForm(step, context) {
    return t.div(
        { className: "automation-graphical-config" },
        t.div(
            { className: "grid" },
            t.div(
                { className: "col-md-7" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${step.__id}_capability` }, "Capability"),
                    app.components.select({
                        id: `${step.__id}_capability`,
                        value: () => step.capability,
                        options: () => capabilitySelectOptions(context.schemas, step.capability),
                        placeholder: "- Select capability -",
                        onchange: (selected) => (step.capability = selected?.[0]?.value || ""),
                    }),
                ),
            ),
            t.div(
                { className: "col-md-5" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${step.__id}_connector` }, "Connector ID"),
                    t.input({
                        id: `${step.__id}_connector`,
                        type: "text",
                        placeholder: "Optional connector",
                        value: () => step.connectorRef,
                        oninput: (e) => (step.connectorRef = e.target.value),
                    }),
                ),
            ),
        ),
        editableStringList({
            title: "Required scopes",
            emptyText: "No extra scopes required.",
            addLabel: "Add scope",
            rows: () => step.requiredScopes,
            add: () => step.requiredScopes.push(""),
            remove: (index) => step.requiredScopes.splice(index, 1),
            move: (from, to) => moveArrayItem(step.requiredScopes, from, to),
            renderValue: (scope, index) =>
                t.input({
                    type: "text",
                    placeholder: "scope:name",
                    value: () => step.requiredScopes[index],
                    oninput: (e) => (step.requiredScopes[index] = e.target.value),
                }),
        }),
        objectRowsEditor({
            title: "Input fields",
            emptyText: "No input fields configured.",
            context,
            rows: () => step.inputRows,
            add: () => step.inputRows.push(createConfigRow()),
            remove: (index) => step.inputRows.splice(index, 1),
            move: (from, to) => moveArrayItem(step.inputRows, from, to),
        }),
        generatedConfigPreview(step),
    );
}

function waitDelayStepForm(step) {
    return t.div(
        { className: "automation-graphical-config" },
        t.div(
            { className: "grid" },
            t.div(
                { className: "col-md-6" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${step.__id}_duration_value` }, "Delay amount"),
                    t.input({
                        id: `${step.__id}_duration_value`,
                        type: "number",
                        min: "1",
                        step: "1",
                        value: () => step.durationValue,
                        oninput: (e) => (step.durationValue = e.target.value),
                    }),
                ),
            ),
            t.div(
                { className: "col-md-6" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${step.__id}_duration_unit` }, "Unit"),
                    app.components.select({
                        id: `${step.__id}_duration_unit`,
                        value: () => step.durationUnit,
                        options: waitDurationUnitOptions,
                        onchange: (selected) => (step.durationUnit = selected?.[0]?.value || "m"),
                    }),
                ),
            ),
        ),
        generatedConfigPreview(step),
    );
}

function waitKeyStepForm(step, context) {
    const label = step.type === "wait.event" ? "Event name" : "Webhook key";
    const placeholder = step.type === "wait.event" ? "invoice.paid" : "payment_completed";
    return t.div(
        { className: "automation-graphical-config" },
        t.div(
            { className: "field" },
            t.label({ htmlFor: `${step.__id}_key` }, label),
            app.components.automationInput({
                id: `${step.__id}_key`,
                singleLine: true,
                placeholder,
                value: () => step.key,
                triggerType: () => context.triggerType,
                triggerCollectionRef: () => context.triggerCollectionRef,
                oninput: (value) => (step.key = value),
            }),
        ),
        generatedConfigPreview(step),
    );
}

function waitApprovalStepForm(step, context) {
    return t.div(
        { className: "automation-graphical-config" },
        t.div(
            { className: "grid" },
            t.div(
                { className: "col-md-6" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${step.__id}_assignee` }, "Assignee"),
                    app.components.automationInput({
                        id: `${step.__id}_assignee`,
                        singleLine: true,
                        placeholder: "user@example.com",
                        value: () => step.assignee,
                        triggerType: () => context.triggerType,
                        triggerCollectionRef: () => context.triggerCollectionRef,
                        oninput: (value) => (step.assignee = value),
                    }),
                ),
            ),
            t.div(
                { className: "col-md-6" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${step.__id}_role` }, "Role"),
                    app.components.automationInput({
                        id: `${step.__id}_role`,
                        singleLine: true,
                        placeholder: "manager",
                        value: () => step.role,
                        triggerType: () => context.triggerType,
                        triggerCollectionRef: () => context.triggerCollectionRef,
                        oninput: (value) => (step.role = value),
                    }),
                ),
            ),
            t.div(
                { className: "col-12" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${step.__id}_approval_comment` }, "Approval note"),
                    app.components.automationInput({
                        id: `${step.__id}_approval_comment`,
                        singleLine: true,
                        placeholder: "Optional note for approvers",
                        value: () => step.comment,
                        triggerType: () => context.triggerType,
                        triggerCollectionRef: () => context.triggerCollectionRef,
                        oninput: (value) => (step.comment = value),
                    }),
                ),
            ),
        ),
        generatedConfigPreview(step),
    );
}

function aiStepForm(step, context) {
    return t.div(
        { className: "automation-graphical-config" },
        t.div(
            { className: "grid" },
            t.div(
                { className: "col-md-5" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${step.__id}_model` }, "Model override"),
                    t.input({
                        id: `${step.__id}_model`,
                        type: "text",
                        placeholder: () => app.store.settings?.ai?.model || "Application AI setting",
                        value: () => step.model,
                        oninput: (e) => (step.model = e.target.value),
                    }),
                ),
            ),
            t.div(
                { className: "col-md-7" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${step.__id}_input` }, "Input"),
                    app.components.automationInput({
                        id: `${step.__id}_input`,
                        singleLine: true,
                        placeholder: "{{record.description}}",
                        value: () => step.inputText,
                        triggerType: () => context.triggerType,
                        triggerCollectionRef: () => context.triggerCollectionRef,
                        oninput: (value) => (step.inputText = value),
                    }),
                ),
            ),
        ),
        () =>
            step.type === "ai.classify"
                ? editableStringList({
                    title: "Classification labels",
                    emptyText: "Add at least one label.",
                    addLabel: "Add label",
                    rows: () => step.labels,
                    add: () => step.labels.push({ __id: app.utils.randomString(), value: "" }),
                    remove: (index) => step.labels.splice(index, 1),
                    move: (from, to) => moveArrayItem(step.labels, from, to),
                    renderValue: (row) =>
                        t.input({
                            type: "text",
                            placeholder: "approved",
                            value: () => row.value,
                            oninput: (e) => (row.value = e.target.value),
                        }),
                })
                : null,
        schemaRowsEditor(step),
        generatedConfigPreview(step),
    );
}

function objectRowsEditor(options) {
    return t.div(
        { className: "automation-config-section" },
        t.div(
            { className: "flex gap-5 flex-wrap m-b-xs" },
            t.div({ className: "txt-bold" }, options.title),
            t.button(
                {
                    type: "button",
                    className: "btn sm secondary transparent m-l-auto",
                    onclick: options.add,
                },
                t.i({ className: "ri-add-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Add field"),
            ),
        ),
        () => {
            const rows = options.rows();
            if (!rows.length) {
                return t.div({ className: "txt-sm txt-hint" }, options.emptyText);
            }

            return t.div(
                { className: "automation-config-rows" },
                ...rows.map((row, index) =>
                    t.div(
                        dragRowAttrs({
                            id: row.__id,
                            index,
                            move: options.move,
                            className: "automation-config-row",
                        }),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm secondary transparent circle automation-config-drag",
                                ariaLabel: app.attrs.tooltip("Drag to reorder"),
                            },
                            t.i({ className: "ri-draggable", ariaHidden: true }),
                        ),
                        t.input({
                            type: "text",
                            placeholder: "Field",
                            value: () => row.key,
                            oninput: (e) => (row.key = e.target.value),
                        }),
                        app.components.select({
                            value: () => row.valueType,
                            options: valueTypeOptions,
                            onchange: (selected) => (row.valueType = selected?.[0]?.value || "text"),
                        }),
                        valueControl(row, options.context),
                        removeRowButton(() => options.remove(index)),
                    )
                ),
            );
        },
    );
}

function editableStringList(options) {
    return t.div(
        { className: "automation-config-section" },
        t.div(
            { className: "flex gap-5 flex-wrap m-b-xs" },
            t.div({ className: "txt-bold" }, options.title),
            t.button(
                {
                    type: "button",
                    className: "btn sm secondary transparent m-l-auto",
                    onclick: options.add,
                },
                t.i({ className: "ri-add-line", ariaHidden: true }),
                t.span({ className: "txt" }, options.addLabel),
            ),
        ),
        () => {
            const rows = options.rows();
            if (!rows.length) {
                return t.div({ className: "txt-sm txt-hint" }, options.emptyText);
            }

            return t.div(
                { className: "automation-config-rows" },
                ...rows.map((row, index) =>
                    t.div(
                        dragRowAttrs({
                            id: row.__id || `${index}_${row}`,
                            index,
                            move: options.move,
                            className: "automation-config-row compact",
                        }),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm secondary transparent circle automation-config-drag",
                                ariaLabel: app.attrs.tooltip("Drag to reorder"),
                            },
                            t.i({ className: "ri-draggable", ariaHidden: true }),
                        ),
                        options.renderValue(row, index),
                        removeRowButton(() => options.remove(index)),
                    )
                ),
            );
        },
    );
}

function schemaRowsEditor(step) {
    return t.div(
        { className: "automation-config-section" },
        t.div(
            { className: "flex gap-5 flex-wrap m-b-xs" },
            t.div({ className: "txt-bold" }, "Output schema"),
            t.button(
                {
                    type: "button",
                    className: "btn sm secondary transparent m-l-auto",
                    onclick: () => step.schemaRows.push(createSchemaRow()),
                },
                t.i({ className: "ri-add-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Add property"),
            ),
        ),
        () => {
            if (!step.schemaRows.length) {
                return t.div({ className: "txt-sm txt-hint" }, "No typed output properties required.");
            }

            return t.div(
                { className: "automation-config-rows" },
                ...step.schemaRows.map((row, index) =>
                    t.div(
                        dragRowAttrs({
                            id: row.__id,
                            index,
                            move: (from, to) => moveArrayItem(step.schemaRows, from, to),
                            className: "automation-config-row schema",
                        }),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm secondary transparent circle automation-config-drag",
                                ariaLabel: app.attrs.tooltip("Drag to reorder"),
                            },
                            t.i({ className: "ri-draggable", ariaHidden: true }),
                        ),
                        t.input({
                            type: "text",
                            placeholder: "Property",
                            value: () => row.key,
                            oninput: (e) => (row.key = e.target.value),
                        }),
                        app.components.select({
                            value: () => row.type,
                            options: schemaTypeOptions,
                            onchange: (selected) => (row.type = selected?.[0]?.value || "string"),
                        }),
                        t.div(
                            { className: "field checkbox-field" },
                            t.input({
                                id: `${row.__id}_required`,
                                type: "checkbox",
                                className: "switch",
                                checked: () => row.required,
                                onchange: (e) => (row.required = e.target.checked),
                            }),
                            t.label({ htmlFor: `${row.__id}_required` }, t.span({ className: "txt" }, "Required")),
                        ),
                        removeRowButton(() => step.schemaRows.splice(index, 1)),
                    )
                ),
            );
        },
    );
}

function valueControl(row, context = {}) {
    if (row.valueType === "boolean") {
        return app.components.select({
            value: () => row.valueText,
            options: [
                { value: "true", label: "True" },
                { value: "false", label: "False" },
            ],
            onchange: (selected) => (row.valueText = selected?.[0]?.value || "false"),
        });
    }

    if (row.valueType === "template") {
        return app.components.automationInput({
            singleLine: true,
            placeholder: "{{record.field}}",
            value: () => row.valueText,
            triggerType: () => context.triggerType,
            triggerCollectionRef: () => context.triggerCollectionRef,
            oninput: (value) => (row.valueText = value),
        });
    }

    return t.input({
        type: row.valueType === "number" ? "number" : "text",
        placeholder: "Value",
        value: () => row.valueText,
        oninput: (e) => (row.valueText = e.target.value),
    });
}

function removeRowButton(onclick) {
    return t.button(
        {
            type: "button",
            className: "btn sm secondary transparent circle txt-danger",
            ariaLabel: app.attrs.tooltip("Remove"),
            onclick,
        },
        t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
    );
}

function dragRowAttrs({ id, index, move, className }) {
    return {
        rid: id,
        className,
        draggable: true,
        "html-data-row-index": index,
        ondragstart: (e) => {
            e.dataTransfer.effectAllowed = "move";
            e.dataTransfer.setData("text/plain", String(index));
        },
        ondragover: (e) => {
            e.preventDefault();
            e.currentTarget.classList.add("drag-over");
        },
        ondragleave: (e) => e.currentTarget.classList.remove("drag-over"),
        ondrop: (e) => {
            e.preventDefault();
            e.currentTarget.classList.remove("drag-over");
            const from = Number(e.dataTransfer.getData("text/plain"));
            if (Number.isInteger(from) && from !== index) {
                move(from, index);
            }
        },
        ondragend: (e) => e.currentTarget.classList.remove("drag-over"),
    };
}

function generatedConfigPreview(step) {
    return t.div(
        { className: "automation-generated-config" },
        t.div({ className: "txt-xs txt-hint m-b-xs" }, "Generated configuration"),
        app.components.codeBlock({
            language: "js",
            value: () => stringifyJSONObject(generatedStepConfig(step), "{}"),
        }),
    );
}

function buildCodePayload(step, index) {
    const code = step.code.trim();
    if (!code) {
        throw new Error(`Step ${index + 1}: JavaScript code is required.`);
    }

    return {
        type: "code",
        code,
    };
}

function buildCapabilityPayload(step, index) {
    const capability = step.capability.trim();
    if (!capability) {
        throw new Error(`Step ${index + 1}: capability key is required.`);
    }

    const payload = {
        type: "capability",
        capability,
    };
    const connectorRef = step.connectorRef.trim();
    if (connectorRef) {
        payload.connectorRef = connectorRef;
    }
    const requiredScopes = normalizeStringArray(step.requiredScopes);
    if (requiredScopes.length) {
        payload.requiredScopes = requiredScopes;
    }

    const input = configRowsToObject(step.inputRows);
    if (Object.keys(input).length) {
        payload.input = input;
    }

    return payload;
}

function buildWaitDelayPayload(step, index) {
    const durationValue = Number(step.durationValue);
    if (!Number.isFinite(durationValue) || durationValue <= 0) {
        throw new Error(`Step ${index + 1}: wait duration must be greater than zero.`);
    }

    return {
        type: "wait.delay",
        duration: `${durationValue}${step.durationUnit || "m"}`,
    };
}

function buildWaitKeyPayload(step, index) {
    const key = step.key.trim();
    if (!key) {
        throw new Error(`Step ${index + 1}: wait key is required.`);
    }

    return {
        type: step.type,
        key,
    };
}

function buildWaitApprovalPayload(step, index) {
    const assignee = step.assignee.trim();
    const role = step.role.trim();
    if (!assignee && !role) {
        throw new Error(`Step ${index + 1}: approval wait requires an assignee or role.`);
    }

    const payload = { type: "wait.approval" };
    if (assignee) {
        payload.assignee = assignee;
    }
    if (role) {
        payload.role = role;
    }
    if (step.comment.trim()) {
        payload.comment = step.comment.trim();
    }

    return payload;
}

function buildAIPayload(step, index) {
    const input = parseLooseValue(step.inputText);
    if (input === "" || input === undefined || input === null) {
        throw new Error(`Step ${index + 1}: AI input is required.`);
    }

    const payload = {
        type: step.type,
        input,
    };
    if (step.model.trim()) {
        payload.model = step.model.trim();
    }

    if (step.type === "ai.classify") {
        const labels = step.labels.map((row) => toString(row.value).trim()).filter(Boolean);
        if (!labels.length) {
            throw new Error(`Step ${index + 1}: AI classify requires at least one label.`);
        }
        payload.labels = labels;
    }

    const schema = schemaRowsToObject(step.schemaRows);
    if (schema) {
        payload.schema = schema;
    }

    return payload;
}

function generatedStepConfig(step) {
    try {
        const payload = buildStepPayload(step, 0);
        const config = { ...payload };
        delete config.type;
        return config;
    } catch (_) {
        return {};
    }
}

function buildConditionPayload(step, index) {
    const conditions = normalizeConditionRows(step);
    if (!conditions.length) {
        throw new Error(`Step ${index + 1}: at least one condition is required.`);
    }

    const payloadConditions = conditions.map((condition, conditionIndex) => {
        const path = condition.path.trim();
        if (!path) {
            throw new Error(`Step ${index + 1}: condition ${conditionIndex + 1} path is required.`);
        }

        const op = condition.op || "exists";
        const payloadCondition = { path, op };

        if (!["exists", "empty", "notEmpty"].includes(op)) {
            const value = parseLooseValue(condition.valueText);
            if (op === "in" && !Array.isArray(value)) {
                throw new Error(
                    `Step ${index + 1}: condition ${
                        conditionIndex + 1
                    } "in" value must be a JSON array or a template-rendered array.`,
                );
            }

            payloadCondition.value = value;
        }

        return payloadCondition;
    });

    if (payloadConditions.length === 1) {
        return {
            type: "condition",
            ...payloadConditions[0],
        };
    }

    return {
        type: "condition",
        match: step.match === "or" ? "or" : "and",
        conditions: payloadConditions,
    };
}

function buildHTTPPayload(step, index) {
    const url = step.url.trim();
    if (!url) {
        throw new Error(`Step ${index + 1}: HTTP url is required.`);
    }

    const payload = {
        type: "http",
        method: (step.method || "GET").trim().toUpperCase(),
        url,
    };

    const headersText = step.headersText.trim();
    if (headersText) {
        payload.headers = parseJSONObject(headersText, `Step ${index + 1}: HTTP headers`);
    }

    const bodyText = step.bodyText.trim();
    if (bodyText) {
        payload.body = parseLooseValue(bodyText);
    }

    const timeoutText = step.timeoutText.trim();
    if (timeoutText) {
        const timeout = Number(timeoutText);
        if (!Number.isFinite(timeout) || timeout <= 0) {
            throw new Error(`Step ${index + 1}: HTTP timeout must be greater than zero.`);
        }

        payload.timeout = timeout;
    }

    return payload;
}

function buildRecordCreatePayload(step, index) {
    const collection = step.collection.trim();
    if (!collection) {
        throw new Error(`Step ${index + 1}: record collection is required.`);
    }

    return {
        type: "record.create",
        collection,
        data: parseJSONObject(step.dataText, `Step ${index + 1}: record data`),
    };
}

function buildMailPayload(step, index) {
    const to = parseStringList(step.toText);
    if (!to.length) {
        throw new Error(`Step ${index + 1}: at least one mail recipient is required.`);
    }

    const subject = step.subject.trim();
    if (!subject) {
        throw new Error(`Step ${index + 1}: mail subject is required.`);
    }

    const text = step.text.trim();
    const html = step.html.trim();
    if (!text && !html) {
        throw new Error(`Step ${index + 1}: mail step requires text or HTML content.`);
    }

    const payload = {
        type: "mail.send",
        to,
        subject,
    };

    const cc = parseStringList(step.ccText);
    if (cc.length) {
        payload.cc = cc;
    }

    const bcc = parseStringList(step.bccText);
    if (bcc.length) {
        payload.bcc = bcc;
    }

    if (text) {
        payload.text = text;
    }

    if (html) {
        payload.html = html;
    }

    const attachments = normalizeStringArray(step.attachments);
    if (attachments.length) {
        payload.attachments = attachments;
    }

    return payload;
}

function buildRecordUpdatePayload(step, index) {
    const payload = buildRecordCreatePayload({
        ...step,
        type: "record.create",
    }, index);

    payload.type = "record.update";

    const id = step.id.trim();
    const filter = step.filter.trim();
    if (!id && !filter) {
        throw new Error(`Step ${index + 1}: record update requires either id or filter.`);
    }

    if (id) {
        payload.id = id;
    }
    if (filter) {
        payload.filter = filter;
    }

    return payload;
}

function buildRecordDeletePayload(step, index) {
    const collection = step.collection.trim();
    if (!collection) {
        throw new Error(`Step ${index + 1}: record collection is required.`);
    }

    const id = step.id.trim();
    const filter = step.filter.trim();
    if (!id && !filter) {
        throw new Error(`Step ${index + 1}: record delete requires either id or filter.`);
    }

    const payload = {
        type: "record.delete",
        collection,
    };

    if (id) {
        payload.id = id;
    }
    if (filter) {
        payload.filter = filter;
    }

    return payload;
}

function buildResponsePayload(step, index) {
    const payload = {
        type: "response",
    };

    const statusCodeText = step.statusCodeText.trim();
    if (statusCodeText) {
        const statusCode = Number(statusCodeText);
        if (!Number.isInteger(statusCode) || statusCode < 100 || statusCode > 599) {
            throw new Error(`Step ${index + 1}: response status code must be an integer between 100 and 599.`);
        }

        payload.statusCode = statusCode;
    }

    const headersText = step.headersText.trim();
    if (headersText) {
        payload.headers = parseJSONObject(headersText, `Step ${index + 1}: response headers`);
    }

    const bodyText = step.bodyText.trim();
    if (bodyText) {
        payload.body = parseLooseValue(bodyText);
    }

    return payload;
}

function summarizeStep(step) {
    switch (step.type) {
        case "condition":
            if (step.conditions?.length > 1) {
                return `${step.conditions.length} conditions • ${(step.match || "and").toUpperCase()}`;
            }

            return `${step.conditions?.[0]?.path || step.path || "Condition path"} • ${
                step.conditions?.[0]?.op || step.op || "exists"
            }`;
        case "code":
            return "Run JavaScript and return object data";
        case "http":
            return `${(step.method || "GET").toUpperCase()} ${step.url || "HTTP request"}`;
        case "mail.send":
            return `Send mail to ${firstStringListValue(step.toText) || "recipient"}`;
        case "record.create":
            return `Create record in ${step.collection || "collection"}`;
        case "record.update":
            return `Update record in ${step.collection || "collection"}`;
        case "record.delete":
            return `Delete record in ${step.collection || "collection"}`;
        case "response":
            return `Return webhook response ${step.statusCodeText || "200"}`;
        case "capability":
            return step.capability ? `Run ${step.capability}` : "Choose a capability";
        case "wait.delay":
            return `Wait ${step.durationValue || "1"} ${durationUnitLabel(step.durationUnit)}`;
        case "wait.webhook":
            return `Wait for webhook ${step.key || "key"}`;
        case "wait.event":
            return `Wait for event ${step.key || "event"}`;
        case "wait.approval":
            return `Approval by ${step.assignee || step.role || "assignee or role"}`;
        case "ai.extract":
        case "ai.classify":
        case "ai.generate":
        case "ai.summarize":
            return step.inputText ? `Use ${step.inputText}` : "Configure AI input";
        default:
            return step.type || "Step";
    }
}

function durationUnitLabel(unit) {
    return waitDurationUnitOptions.find((option) => option.value === unit)?.label.toLowerCase() || "minutes";
}

function clientValidateStep(step) {
    const messages = [];

    switch (step.type) {
        case "condition":
            normalizeConditionRows(step).forEach((condition, index) => {
                if (!condition.path?.trim()) {
                    messages.push(`Condition ${index + 1} path is required.`);
                }
                if (!["exists", "empty", "notEmpty"].includes(condition.op) && !condition.valueText?.trim()) {
                    messages.push(`Condition ${index + 1} value is required for this operator.`);
                }
            });
            if (step.match && !["and", "or"].includes(step.match)) {
                messages.push("Condition match must be AND or OR.");
            }
            break;
        case "http":
            if (!step.url?.trim()) {
                messages.push("HTTP URL is required.");
            }
            validateOptionalJSONObject(step.headersText, "HTTP headers", messages);
            validateOptionalNumber(step.timeoutText, "HTTP timeout", messages);
            break;
        case "mail.send":
            if (!parseStringList(step.toText || "").length) {
                messages.push("At least one mail recipient is required.");
            }
            if (!step.subject?.trim()) {
                messages.push("Mail subject is required.");
            }
            if (!step.text?.trim() && !step.html?.trim()) {
                messages.push("Mail text or HTML body is required.");
            }
            break;
        case "record.create":
            if (!step.collection?.trim()) {
                messages.push("Record collection is required.");
            }
            validateJSONObject(step.dataText, "Record data", messages);
            break;
        case "record.update":
            if (!step.collection?.trim()) {
                messages.push("Record collection is required.");
            }
            if (!step.id?.trim() && !step.filter?.trim()) {
                messages.push("Record update requires an id or filter.");
            }
            validateJSONObject(step.dataText, "Record data", messages);
            break;
        case "record.delete":
            if (!step.collection?.trim()) {
                messages.push("Record collection is required.");
            }
            if (!step.id?.trim() && !step.filter?.trim()) {
                messages.push("Record delete requires an id or filter.");
            }
            break;
        case "response":
            validateOptionalNumber(step.statusCodeText, "Response status code", messages);
            validateOptionalJSONObject(step.headersText, "Response headers", messages);
            break;
        case "code":
            if (!step.code?.trim()) {
                messages.push("JavaScript code is required.");
            }
            break;
        case "capability":
        case "wait.delay":
        case "wait.webhook":
        case "wait.event":
        case "wait.approval":
        case "ai.extract":
        case "ai.classify":
        case "ai.generate":
        case "ai.summarize":
            validateGenericStepConfig(step, messages);
            break;
    }

    return messages;
}

function validateJSONObject(raw, label, messages) {
    try {
        parseJSONObject(raw || "", label);
    } catch (err) {
        messages.push(err.message);
    }
}

function validateOptionalJSONObject(raw, label, messages) {
    if (!raw?.trim()) {
        return;
    }

    validateJSONObject(raw, label, messages);
}

function validateOptionalNumber(raw, label, messages) {
    if (!raw?.trim()) {
        return;
    }

    const value = Number(raw);
    if (!Number.isFinite(value) || value <= 0) {
        messages.push(`${label} must be greater than zero.`);
    }
}

function validateGenericStepConfig(step, messages) {
    switch (step.type) {
        case "capability":
            if (!step.capability?.trim()) {
                messages.push("Capability key is required.");
            }
            validateConfigRows(step.inputRows, "Capability input", messages);
            break;
        case "wait.delay":
            if (!String(step.durationValue || "").trim()) {
                messages.push("Wait duration is required.");
            } else if (!Number.isFinite(Number(step.durationValue)) || Number(step.durationValue) <= 0) {
                messages.push("Wait duration must be greater than zero.");
            }
            break;
        case "wait.webhook":
        case "wait.event":
            if (!step.key?.trim()) {
                messages.push("Wait key is required.");
            }
            break;
        case "wait.approval":
            if (!step.assignee?.trim() && !step.role?.trim()) {
                messages.push("Approval assignee or role is required.");
            }
            break;
        case "ai.extract":
        case "ai.classify":
        case "ai.generate":
        case "ai.summarize":
            if (!step.inputText?.trim()) {
                messages.push("AI input is required.");
            }
            if (step.type === "ai.classify" && !step.labels.some((row) => row.value?.trim())) {
                messages.push("AI classify labels are required.");
            }
            validateSchemaRows(step.schemaRows, messages);
            break;
    }
}

function validateConfigRows(rows, label, messages) {
    const seen = new Set();
    for (const row of rows || []) {
        const key = row.key?.trim();
        if (!key && row.valueText?.trim()) {
            messages.push(`${label} field name is required.`);
        }
        if (!key) {
            continue;
        }
        if (seen.has(key)) {
            messages.push(`${label} field "${key}" is duplicated.`);
        }
        seen.add(key);
        if (row.valueType === "number" && (!Number.isFinite(Number(row.valueText)) || row.valueText === "")) {
            messages.push(`${label} field "${key}" must be a number.`);
        }
    }
}

function validateSchemaRows(rows, messages) {
    const seen = new Set();
    for (const row of rows || []) {
        const key = row.key?.trim();
        if (!key) {
            continue;
        }
        if (seen.has(key)) {
            messages.push(`Output schema property "${key}" is duplicated.`);
        }
        seen.add(key);
    }
}

function stepTypeLabel(type) {
    return stepTypeOptions.find((option) => option.value === type)?.label || type || "Step";
}

function stepTypeIcon(type) {
    return stepTypeOptions.find((option) => option.value === type)?.icon || "ri-git-branch-line";
}

function filteredCapabilities(capabilities, options) {
    const query = (options.capabilityQuery || "").trim().toLowerCase();
    const category = options.capabilityCategory || "";

    return capabilities
        .filter((capability) => !category || capability.category === category)
        .filter((capability) => {
            if (!query) {
                return true;
            }

            return [
                capability.key,
                capability.category,
                capability.version,
                capability.authStrategy,
            ].some((value) => String(value || "").toLowerCase().includes(query));
        })
        .sort((a, b) => String(a.key || "").localeCompare(String(b.key || "")));
}

function resolveStepListError(errors) {
    if (!errors || typeof errors !== "object") {
        return null;
    }

    const keys = Object.keys(errors).filter((key) => !/^\d+$/.test(key));
    if (!keys.length) {
        return null;
    }

    return errors;
}

function resolveStepError(errors, index) {
    if (!errors || typeof errors !== "object") {
        return null;
    }

    return errors[index] || errors[String(index)] || null;
}

function extractErrorMessage(value) {
    if (!value) {
        return "";
    }

    if (typeof value?.message === "string") {
        return value.message;
    }

    if (Array.isArray(value)) {
        return value.map((entry) => extractErrorMessage(entry)).filter(Boolean).join("\n");
    }

    if (typeof value === "object") {
        const nested = Object.values(value).map((entry) => extractErrorMessage(entry)).filter(Boolean);
        if (nested.length) {
            return nested[0];
        }
    }

    return String(value || "");
}

function resetStep(target, source) {
    for (const key of Object.keys(target)) {
        delete target[key];
    }

    Object.assign(target, source);
}

function parseJSONObject(raw, label) {
    let parsed;
    try {
        parsed = JSON.parse(raw);
    } catch (_) {
        throw new Error(`${label} must be valid JSON.`);
    }

    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        throw new Error(`${label} must be a JSON object.`);
    }

    return parsed;
}

function parseLooseValue(raw) {
    const trimmed = raw.trim();
    if (!trimmed) {
        return "";
    }

    try {
        return JSON.parse(trimmed);
    } catch (_) {
        return raw;
    }
}

function stringifyJSONObject(value, fallback = "{}") {
    if (value === undefined || value === null || value === "") {
        return fallback;
    }

    if (typeof value === "string") {
        try {
            value = JSON.parse(value);
        } catch (_) {
            return value;
        }
    }

    try {
        return JSON.stringify(value, null, 2);
    } catch (_) {
        return fallback;
    }
}

function stringifyLooseValue(value) {
    if (value === undefined || value === null) {
        return "";
    }

    if (typeof value === "string") {
        return value;
    }

    try {
        return JSON.stringify(value, null, 2);
    } catch (_) {
        return String(value);
    }
}

function stringifyStringArray(value) {
    return normalizeStringArray(value).join("\n");
}

function createConditionRow(raw = {}) {
    return {
        __id: raw.__id || app.utils.randomString(),
        path: toString(raw.path || raw.field),
        op: toString(raw.op) || "exists",
        valueText: raw.value === undefined ? toString(raw.valueText) : stringifyLooseValue(raw.value),
    };
}

function normalizeConditionRows(rawStep = {}) {
    if (Array.isArray(rawStep.conditions) && rawStep.conditions.length) {
        return rawStep.conditions.map((condition) => createConditionRow(condition));
    }

    return [createConditionRow(rawStep)];
}

function normalizeStringArray(value) {
    if (!Array.isArray(value)) {
        return [];
    }

    return value
        .map((item) => toString(item).trim())
        .filter(Boolean);
}

function parseStringList(raw) {
    return raw
        .split(/\r?\n|,/)
        .map((item) => item.trim())
        .filter(Boolean);
}

function firstStringListValue(raw) {
    return parseStringList(raw || "")[0] || "";
}

function objectToConfigRows(value) {
    if (!value || typeof value !== "object" || Array.isArray(value)) {
        return [];
    }

    return Object.keys(value).map((key) => {
        const row = createConfigRow();
        row.key = key;
        const item = value[key];
        if (typeof item === "number") {
            row.valueType = "number";
            row.valueText = String(item);
        } else if (typeof item === "boolean") {
            row.valueType = "boolean";
            row.valueText = item ? "true" : "false";
        } else if (typeof item === "string" && item.includes("{{")) {
            row.valueType = "template";
            row.valueText = item;
        } else {
            row.valueType = "text";
            row.valueText = item === undefined || item === null ? "" : String(item);
        }
        return row;
    });
}

function createConfigRow() {
    return {
        __id: app.utils.randomString(),
        key: "",
        valueType: "text",
        valueText: "",
    };
}

function configRowsToObject(rows = []) {
    const result = {};
    for (const row of rows) {
        const key = toString(row.key).trim();
        if (!key) {
            continue;
        }
        result[key] = configRowValue(row);
    }
    return result;
}

function configRowValue(row) {
    switch (row.valueType) {
        case "number": {
            const value = Number(row.valueText);
            return Number.isFinite(value) ? value : 0;
        }
        case "boolean":
            return row.valueText === "true";
        case "template":
        case "text":
        default:
            return toString(row.valueText);
    }
}

function parseDurationParts(duration) {
    const raw = toString(duration).trim();
    const match = raw.match(/^(\d+(?:\.\d+)?)(ms|s|m|h|d)?$/);
    if (!match) {
        return { value: raw ? raw.replace(/[^\d.]/g, "") || "1" : "1", unit: "m" };
    }

    if (match[2] === "d") {
        return {
            value: String(Number(match[1] || "1") * 24),
            unit: "h",
        };
    }

    return {
        value: match[1] || "1",
        unit: match[2] === "ms" ? "s" : match[2] || "m",
    };
}

function schemaToRows(schema) {
    const properties = schema?.properties;
    if (!properties || typeof properties !== "object" || Array.isArray(properties)) {
        return [];
    }

    const required = Array.isArray(schema.required) ? schema.required.map((item) => toString(item)) : [];
    return Object.keys(properties).map((key) => {
        const prop = properties[key] || {};
        return {
            __id: app.utils.randomString(),
            key,
            type: toString(prop.type) || "string",
            required: required.includes(key),
        };
    });
}

function createSchemaRow(raw = {}) {
    return {
        __id: app.utils.randomString(),
        key: toString(raw.key),
        type: toString(raw.type) || "string",
        required: !!raw.required,
    };
}

function schemaRowsToObject(rows = []) {
    const properties = {};
    const required = [];
    for (const row of rows) {
        const key = toString(row.key).trim();
        if (!key) {
            continue;
        }

        properties[key] = { type: row.type || "string" };
        if (row.required) {
            required.push(key);
        }
    }

    if (!Object.keys(properties).length) {
        return null;
    }

    const schema = {
        type: "object",
        properties,
    };
    if (required.length) {
        schema.required = required;
    }

    return schema;
}

function moveArrayItem(items, from, to) {
    if (!Array.isArray(items) || from < 0 || to < 0 || from >= items.length || to >= items.length || from === to) {
        return;
    }

    const [item] = items.splice(from, 1);
    items.splice(to, 0, item);
}

function capabilitySelectOptions(schemas, selectedValue = "") {
    const capabilities = Object.values(schemas?.capabilities || {})
        .map((capability) => ({
            value: capability.key,
            label: `${capability.key} (${capability.category || "capability"})`,
        }))
        .sort((a, b) => a.label.localeCompare(b.label));

    if (selectedValue && !capabilities.find((option) => option.value === selectedValue)) {
        capabilities.unshift({ value: selectedValue, label: selectedValue });
    }

    return capabilities;
}

function toString(value) {
    if (value === undefined || value === null) {
        return "";
    }

    return String(value);
}
