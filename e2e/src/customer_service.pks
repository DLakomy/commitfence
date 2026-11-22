CREATE OR REPLACE PACKAGE app.customer_service AS
  --@commitfence
  PROCEDURE save_customer(p_external_id NUMBER);
END customer_service;
/
