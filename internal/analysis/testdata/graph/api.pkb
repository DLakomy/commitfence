CREATE PACKAGE BODY app.api AS
  PROCEDURE advertised IS BEGIN NULL; END advertised;
  FUNCTION overloaded(n NUMBER) RETURN NUMBER IS BEGIN COMMIT; RETURN 1; END overloaded;
  FUNCTION overloaded RETURN NUMBER IS BEGIN ROLLBACK; RETURN 2; END overloaded;
  PROCEDURE clean_a IS BEGIN clean_b; END clean_a;
  PROCEDURE clean_b IS BEGIN clean_a; END clean_b;
  PROCEDURE cycle_a IS BEGIN cycle_b; END cycle_a;
  PROCEDURE cycle_b IS BEGIN cycle_a; COMMIT; END cycle_b;
  FUNCTION leaf RETURN NUMBER IS BEGIN COMMIT; RETURN 1; END leaf;
  PROCEDURE left_path IS BEGIN leaf(); END left_path;
  PROCEDURE right_path IS BEGIN leaf(); END right_path;
  PROCEDURE diamond IS BEGIN left_path; right_path; END diamond;
  PROCEDURE expressions IS
    n NUMBER;
  BEGIN
    n := overloaded(1);
    SELECT app.api.leaf() INTO n FROM dual WHERE app.api.leaf() > 0;
    INSERT INTO app.results(n) VALUES (app.api.leaf());
    UPDATE app.results SET n = app.api.leaf();
    DELETE FROM app.results WHERE n = app.api.leaf();
    advertised;
  END expressions;
END api;
/
