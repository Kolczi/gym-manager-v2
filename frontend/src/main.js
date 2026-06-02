import { HealthService } from "../bindings/gym-manager-v2";

var el = document.getElementById("db-status");

HealthService.HealthCheck()
    .then(function (result) {
        el.textContent = result;
        el.classList.add(result.indexOf("\u2713") >= 0 ? "ok" : "err");
    })
    .catch(function (err) {
        el.textContent = "Błąd: " + err;
        el.classList.add("err");
    });
