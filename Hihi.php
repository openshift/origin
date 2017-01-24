<?php
 
/* == ID tài khoản muốn tăng share == */
$user = 'Truong.Cong.Trua';
/* == Token tài khoản chứa page == */
$token = 'EAAAAAYsX7TsBABZC3ZB8vc0PJCZBxwjAPS85orCAZCSZBmG9gLZAktUKSdQJnaUTJX4No7ZAoaatjuVk3F8KdaTWeepaPPfn0DqCuFvXgzE0NgOmfDiOzIeN7bZAwdtbfiklVke65jfJAutsZASQjp9Cm0xcWHNxpZAhU4FrqJaxgMzboiDA8ZC2RWj2OxU33pzSvA1qPprZAXlZAPwZDZD';
$accounts = json_decode(cURL('https://graph.facebook.com/me/accounts?access_token=' . $token),true);
 
$feed = json_decode(cURL('https://graph.facebook.com/' . $user . '/feed?access_token='.$token.'&limit=1'),true);
 
foreach ($accounts['data'] as $data) {
    //echo $data['access_token'] . '<br/>';
    echo cURL('https://graph.facebook.com/' . $feed['data'][0]['id'] . '/sharedposts?method=post&access_token='.$data['access_token']) . '<br/><br/><br/>';
}
 
/* == Hàm get == */
function cURL ($url) {
    $data = curl_init();
    curl_setopt($data, CURLOPT_RETURNTRANSFER, 1);
    curl_setopt($data, CURLOPT_URL, $url);
    $hasil = curl_exec($data);
    curl_close($data);
    return $hasil;
}
 
?>
 
<meta http-equiv="refresh" content="0">Truong.Cong.Trua
