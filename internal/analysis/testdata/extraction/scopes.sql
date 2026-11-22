CREATE OR REPLACE PACKAGE app.worker AS
  --@commitfence
  PROCEDURE helper;
END worker;
/
CREATE OR REPLACE PROCEDURE app.worker IS
  PROCEDURE helper IS
  BEGIN
    COMMIT;
  END helper;
BEGIN
  NULL;
END worker;
/
