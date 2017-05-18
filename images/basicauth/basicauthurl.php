<?php

header('Content-Type: application/json');
$user = $_SERVER['REMOTE_USER'];
$data = array( 'sub' => $user );
echo json_encode($data);

?>
