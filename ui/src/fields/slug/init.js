import { input } from "./input";
import { settings } from "./settings";
import { view } from "./view";

window.app = window.app || {};
window.app.fieldTypes = window.app.fieldTypes || {};
window.app.fieldTypes.slug = {
    icon: "ri-hashtag",
    label: "Slug",
    settings,
    input,
    view,
    filterModifiers: () => {
        return ["lower"];
    },
    dummyData: () => {
        return app.utils.slugify("example text", "-").replaceAll("_", "-");
    },
};
