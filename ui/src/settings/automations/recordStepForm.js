const dataAutofillByStep = new WeakMap();

export function recordStepForm(propsArg = {}) {
    const props = store({
        step: null,
        error: null,
        triggerType: "",
        triggerCollectionRef: "",
    });

    const watchers = app.utils.extendStore(props, propsArg);
    function maybeAutofillDataText() {
        if (props.step.type === "record.delete") {
            return;
        }

        const state = dataAutofillState(props.step);
        const collection = findCollection(props.step.collection);
        if (!collection) {
            return;
        }

        const nextDataText = buildDefaultDataText(collection);
        const currentText = (props.step.dataText || "").trim();

        if (state.userEdited && currentText !== state.lastAutofillText) {
            return;
        }

        if (!currentText || currentText === "{}" || currentText === state.lastAutofillText) {
            props.step.dataText = nextDataText;
            state.lastAutofillText = nextDataText;
            state.userEdited = false;
        }
    }

    return t.div(
        {
            className: "grid automation-record-step-form",
            onmount: () => {
                watchers.push(
                    watch(
                        () => `${props.step.type}::${props.step.collection}`,
                        () => maybeAutofillDataText(),
                    ),
                );

                maybeAutofillDataText();
            },
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "col-md-6" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_collection` }, "Collection"),
                app.components.select({
                    id: `${props.step.__id}_collection`,
                    value: () => props.step.collection,
                    options: () => collectionOptions(props.step.collection),
                    placeholder: "- Select collection -",
                    onchange: (selected) => {
                        props.step.collection = selected?.[0]?.value || "";
                    },
                }),
            ),
        ),
        app.components.slide(
            () => props.step.type !== "record.create",
            t.div(
                { className: "col-md-3" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${props.step.__id}_id` }, "Record ID"),
                    app.components.automationInput({
                        id: `${props.step.__id}_id`,
                        singleLine: true,
                        placeholder: "{{record.id}}",
                        value: () => props.step.id,
                        triggerType: () => props.triggerType,
                        triggerCollectionRef: () => props.triggerCollectionRef,
                        oninput: (value) => (props.step.id = value),
                    }),
                ),
            ),
        ),
        app.components.slide(
            () => props.step.type !== "record.create",
            t.div(
                { className: "col-md-3" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${props.step.__id}_filter` }, "Filter"),
                    app.components.automationInput({
                        id: `${props.step.__id}_filter`,
                        singleLine: true,
                        placeholder: `status = "pending"`,
                        value: () => props.step.filter,
                        triggerType: () => props.triggerType,
                        triggerCollectionRef: () => props.triggerCollectionRef,
                        oninput: (value) => (props.step.filter = value),
                    }),
                ),
            ),
        ),
        app.components.slide(
            () => props.step.type !== "record.delete",
            t.div(
                { className: "col-lg-12" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: `${props.step.__id}_data` }, "Data JSON"),
                    app.components.automationInput({
                        id: `${props.step.__id}_data`,
                        className: "txt-code",
                        language: "js",
                        value: () => props.step.dataText,
                        triggerType: () => props.triggerType,
                        triggerCollectionRef: () => props.triggerCollectionRef,
                        placeholder: () => defaultPlaceholder(props.step.collection),
                        oninput: (value) => {
                            props.step.dataText = value;
                            dataAutofillState(props.step).userEdited = true;
                        },
                    }),
                ),
                t.div(
                    { className: "field-help" },
                    "Data must be a JSON object. Selecting a collection will prefill this block from the collection fields.",
                ),
            ),
        ),
        () => {
            if (props.step.type === "record.create") {
                return t.div(
                    { className: "col-lg-12 field-help" },
                    "Create steps insert a new record in the selected collection using the rendered data object.",
                );
            }

            if (props.step.type === "record.update") {
                return t.div(
                    { className: "col-lg-12 field-help" },
                    "Update steps require either a record ID or a filter. If both are provided, the ID path wins.",
                );
            }

            return t.div(
                { className: "col-lg-12 field-help" },
                "Delete steps require either a record ID or a filter.",
            );
        },
    );
}

function dataAutofillState(step) {
    let state = dataAutofillByStep.get(step);

    if (!state) {
        state = {
            lastAutofillText: "",
            userEdited: false,
        };
        dataAutofillByStep.set(step, state);
    }

    return state;
}

function collectionOptions(selectedValue) {
    const options = (app.store.collections || [])
        .filter((collection) => collection?.type === "base" || collection?.type === "auth")
        .map((collection) => ({
            value: collection.id,
            label: `${collection.name} (${collection.type})`,
        }));

    if (selectedValue && !options.find((option) => option.value === selectedValue)) {
        options.unshift({
            value: selectedValue,
            label: selectedValue,
        });
    }

    return options;
}

function findCollection(idOrName) {
    return (app.store.collections || []).find((collection) => {
        return collection?.id === idOrName || collection?.name === idOrName;
    }) || null;
}

function defaultPlaceholder(collectionIdOrName) {
    const collection = findCollection(collectionIdOrName);
    if (!collection) {
        return `{\n  "title": "{{record.title}}"\n}`;
    }

    return buildDefaultDataText(collection);
}

function buildDefaultDataText(collection) {
    return JSON.stringify(buildDefaultDataObject(collection), null, 2);
}

function buildDefaultDataObject(collection) {
    const result = {};
    const fields = collection?.fields || [];

    for (const field of fields) {
        if (shouldSkipDefaultField(field)) {
            continue;
        }

        let value;
        if (app.fieldTypes[field.type]?.dummyData) {
            value = app.fieldTypes[field.type].dummyData(field, false);
        } else {
            value = "[[DATA]]";
        }

        if (typeof value !== "undefined") {
            result[field.name] = value;
        }
    }

    return result;
}

function shouldSkipDefaultField(field) {
    return !!(
        !field
        || field.hidden
        || field.primaryKey
        || field.type === "autodate"
    );
}
