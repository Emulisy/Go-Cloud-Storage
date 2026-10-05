// Page entry point: views render UI; requests own HTTP; controllers connect them.
import { initNavigation } from "./views/navigation.js?v=14";
import { initFormValidation } from "./views/forms.js?v=14";
import { initSession } from "./controllers/session.js?v=14";
import { initAuth } from "./controllers/auth.js?v=14";
import { initAccount } from "./controllers/account.js?v=14";
import { initFiles } from "./controllers/files.js?v=14";
import { initUpload } from "./controllers/upload.js?v=14";

const pageName = document.body.dataset.page;
const landingPage = document.body.classList.contains("welcome-page");
initNavigation();
initFormValidation();
initSession(pageName, landingPage);
initAuth();
initAccount(pageName);
const loadFiles = initFiles(pageName);
initUpload(pageName, loadFiles);
