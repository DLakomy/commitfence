CREATE PACKAGE app.api AS
  --@commitfence
  PROCEDURE advertised;
  --@commitfence
  FUNCTION overloaded(n NUMBER) RETURN NUMBER;
  PROCEDURE missing_body;
END api;
/
