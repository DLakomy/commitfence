--@commitfence
CREATE OR REPLACE PACKAGE app.bad AS
  PROCEDURE declaration_only;
END bad;
/
CREATE OR REPLACE PROCEDURE app.unnamed IS
BEGIN
  COMMIT;
END;
/
CREATE OR REPLACE PROCEDURE app.wrong IS
BEGIN
  COMMIT;
END somebody_else;
/
CREATE OR REPLACE PROCEDURE app.recovered IS
BEGIN
  ROLLBACK;
END recovered;
/
CREATE OR REPLACE PROCEDURE app.host IS
  PROCEDURE broken IS
  BEGIN
    NULL;
BEGIN
  NULL;
END host;
/
--@commitfence
x NUMBER;
CREATE OR REPLACE PROCEDURE app.unattached IS
BEGIN
  COMMIT;
END unattached;
/
--@commitfence
