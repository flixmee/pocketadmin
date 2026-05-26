export function mailStepForm(propsArg = {}) {
    const props = store({
        step: null,
        error: null,
        triggerType: "",
        triggerCollectionRef: "",
    });

    const watchers = app.utils.extendStore(props, propsArg);

    const uniqueId = "mail_step_form_" + app.utils.randomString();

    return t.div(
        {
            className: "grid automation-mail-step-form",
            onmount: () => {
                watchers.push(
                    watch(
                        () => `${props.triggerType}::${props.triggerCollectionRef}`,
                        () => pruneInvalidAttachments(),
                    ),
                );

                pruneInvalidAttachments();
            },
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "col-lg-12 field-help" },
            "From: ",
            () => formatDefaultSender(),
        ),
        addressField("to", "To", "customer@example.com", true),
        addressField("cc", "Cc", "manager@example.com"),
        addressField("bcc", "Bcc", "audit@example.com"),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_subject` }, "Subject"),
                app.components.automationInput({
                    id: `${props.step.__id}_subject`,
                    singleLine: true,
                    placeholder: "Order {{record.id}} is ready",
                    value: () => props.step.subject,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step.subject = value),
                }),
            ),
        ),
        t.div(
            { className: "col-lg-6" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_text` }, "Text body"),
                app.components.automationInput({
                    id: `${props.step.__id}_text`,
                    className: "pre-wrap",
                    placeholder: "Your order {{record.id}} is ready.",
                    value: () => props.step.text,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step.text = value),
                }),
            ),
        ),
        t.div(
            { className: "col-lg-6" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_html` }, "HTML body"),
                app.components.automationInput({
                    id: `${props.step.__id}_html`,
                    className: "txt-code pre-wrap",
                    language: "html",
                    placeholder: "<p>Your order <strong>{{record.id}}</strong> is ready.</p>",
                    value: () => props.step.html,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step.html = value),
                }),
            ),
        ),
        t.div(
            { className: "col-lg-12 field-help" },
            "Provide text, HTML, or both. Template placeholders such as ",
            t.code(null, "{{record.email}}"),
            " and ",
            t.code(null, "{{automation.name}}"),
            " are supported.",
        ),
        t.div(
            { className: "col-lg-12" },
            t.div({ className: "txt-bold m-b-sm" }, "Record attachments"),
            () => {
                if (!supportsRecordAttachments(props.triggerType)) {
                    return t.div(
                        { className: "field-help" },
                        "Attachments can be picked only for record create/update triggers.",
                    );
                }

                const fields = fileFieldOptions(props.triggerCollectionRef);
                if (!fields.length) {
                    return t.div(
                        { className: "field-help" },
                        "The selected trigger collection has no file fields to attach.",
                    );
                }

                return t.div(
                    { className: "grid" },
                    () =>
                        fields.map((field) => {
                            const checked = () => props.step.attachments.includes(field.name);

                            return t.div(
                                {
                                    rid: field.name,
                                    className: "col-md-6",
                                },
                                t.div(
                                    { className: "field" },
                                    t.input({
                                        type: "checkbox",
                                        className: "sm",
                                        id: `${props.step.__id}_attach_${field.name}`,
                                        checked: () => checked(),
                                        onchange: (e) => {
                                            props.step.attachments = toggleAttachment(
                                                props.step.attachments,
                                                field.name,
                                                e.target.checked,
                                            );
                                        },
                                    }),
                                    t.label(
                                        { className: "txt", htmlFor: `${props.step.__id}_attach_${field.name}` },
                                        field.name,
                                        field.maxSelect > 1 ? " (multiple files)" : "",
                                    ),
                                ),
                            );
                        }),
                );
            },
        ),
        t.div(
            { className: "col-lg-12 field-help" },
            "When selected, all files from the checked file fields on the trigger record are attached to the outgoing email.",
        ),
    );

    function pruneInvalidAttachments() {
        if (!supportsRecordAttachments(props.triggerType)) {
            if (props.step.attachments.length) {
                props.step.attachments = [];
            }
            return;
        }

        const allowed = new Set(fileFieldOptions(props.triggerCollectionRef).map((field) => field.name));
        props.step.attachments = props.step.attachments.filter((name) => allowed.has(name));
    }

    function addressField(key, label, placeholder, required = false) {
        return t.div(
            { className: "col-md-4" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_${key}` }, label),
                app.components.automationInput({
                    id: `${props.step.__id}_${key}`,
                    className: "pre-wrap mini",
                    placeholder,
                    value: () => props.step[key + "Text"],
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step[key + "Text"] = value),
                }),
            ),
            t.div(
                { className: "field-help" },
                required
                    ? "Required. Use one email per line or separate with commas."
                    : "Optional. Use one email per line or separate with commas.",
            ),
        );
    }
}

function formatDefaultSender() {
    const meta = app.store.settings?.meta || {};
    const senderName = (meta.senderName || "").trim();
    const senderAddress = (meta.senderAddress || "").trim();

    if (senderName && senderAddress) {
        return `${senderName} <${senderAddress}>`;
    }

    return senderAddress || senderName || "Mail settings sender";
}

function supportsRecordAttachments(triggerType) {
    return triggerType === "record.beforeCreate"
        || triggerType === "record.beforeUpdate"
        || triggerType === "record.create"
        || triggerType === "record.update";
}

function fileFieldOptions(collectionIdOrName) {
    const collection = (app.store.collections || []).find((item) => {
        return item?.id === collectionIdOrName || item?.name === collectionIdOrName;
    });

    return (collection?.fields || [])
        .filter((field) => field?.type === "file" && !field.hidden)
        .map((field) => ({
            name: field.name,
            maxSelect: field.maxSelect || 1,
        }));
}

function toggleAttachment(attachments, name, checked) {
    const next = Array.isArray(attachments) ? [...attachments] : [];
    const index = next.indexOf(name);

    if (checked && index === -1) {
        next.push(name);
    } else if (!checked && index >= 0) {
        next.splice(index, 1);
    }

    return next;
}
