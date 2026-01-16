<?php
header("Content-Type: text/plain");
foreach ($_POST as $key => $value) {
    if (md5($value) === $key) {
        echo "-PASSED-";
        exit;
    }
}
echo "-FAILED-";
