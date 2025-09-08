<?php
class AdminerLoginServers {
    function __construct() {
        $this->servers = [
            "LocalSU" => $this->parseEnv('SERVICE_PGSQL_DBURL_SU'),
            "APIUser" => $this->parseEnv('SERVICE_PGSQL_DBURL_API'),
            "HMUser" => $this->parseEnv('SERVICE_PGSQL_DBURL_HM'),
        ];

        if (isset($_POST["auth"]["custom_server"]) && $_POST["auth"]["custom_server"]) {
            $serverKey = $_POST["auth"]["custom_server"];
            $_POST["auth"] = array_merge($_POST["auth"], $this->servers[$serverKey]);
        }
    }

    private function parseEnv($env) {
        $url = getenv($env);
        if (!$url) {
            return [];
        }

        $parts = parse_url($url);
        if (!$parts) {
            echo "Invalid URL: {$url}";
            return [];
        }

        return [
            'driver' => 'pgsql',
            'server' => $parts['host'] . (isset($parts['port']) ? ':' . $parts['port'] : ''),
            'username' => $parts['user'],
            'password' => isset($parts['pass']) ? urldecode($parts['pass']) : '',
            'db' => trim($parts['path'], '/')
        ];
    }

    function loginFormField($name, $heading, $value) {
        if ($name == 'driver') {
            return "<tr><th>Driver</th><td>" . $value . "</td></tr>";
        }
        elseif ($name == 'server') {
            return "<tr><th>Host</th><td>" . $value . "</td></tr>";
        }
        elseif ($name == 'db') {
            $out = "<tr><th>Database</th><td>" . $value . "</td></tr>";
            $out .= "<tr><td colspan='2' style='text-align: center; padding: 5px 0;'>or</td></tr>";
            $out .= "<tr><th>Environment</th><td><select name='auth[custom_server]' style='width: 100%;'>";
            $out .= "<option value='' selected>-- Choose Environment --</option>";
            foreach ($this->servers as $serverName => $serverConfig) {
                $out .= "<option value='" . htmlspecialchars($serverName) . "'>" . htmlspecialchars($serverName) . "</option>";
            }
            $out .= "</select></td></tr>";
            return $out;
        }
        return null;
    }
}

return new AdminerLoginServers();
?>
