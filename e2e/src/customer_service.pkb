CREATE OR REPLACE PACKAGE BODY app.customer_service AS
  PROCEDURE save_customer(p_external_id NUMBER) IS
    customer_id NUMBER;
  BEGIN
    customer_id := ext.customer_api.find_customer(p_external_id);
    COMMIT;
  END save_customer;
END customer_service;
/
