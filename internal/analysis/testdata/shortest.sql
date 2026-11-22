CREATE PROCEDURE app.leaf IS BEGIN COMMIT; END leaf;
/
CREATE PROCEDURE app.long_path IS BEGIN app.leaf; END long_path;
/
CREATE PROCEDURE app.branch IS BEGIN app.long_path; app.leaf; END branch;
/
CREATE PROCEDURE app.upstream IS BEGIN app.branch; END upstream;
/
