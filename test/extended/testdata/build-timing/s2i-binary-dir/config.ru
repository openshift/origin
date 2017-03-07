run Proc.new {|env| [200, {"Content-Type" => "text/html"}, [ENV['TEST_ENV']]]}
