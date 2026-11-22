CREATE PROCEDURE app.top IS BEGIN app.api.diamond; END top;
/
CREATE PROCEDURE other.advertised IS BEGIN COMMIT; END advertised;
/
CREATE PROCEDURE app.unrelated IS BEGIN advertised; END unrelated;
/
CREATE PROCEDURE app.unknown_wrapper IS BEGIN app.api.missing_body; END unknown_wrapper;
/
--@commitfence
CREATE PROCEDURE app.permitted_wrapper IS BEGIN app.unknown_wrapper; app.api.leaf(); END permitted_wrapper;
/
CREATE PROCEDURE app.capability_caller IS BEGIN app.api.advertised; END capability_caller;
/
CREATE PROCEDURE app.unknown_caller IS BEGIN app.permitted_wrapper; END unknown_caller;
/
