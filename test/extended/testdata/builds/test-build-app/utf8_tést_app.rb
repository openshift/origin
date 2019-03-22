require 'rack'
 
app = Proc.new do |env|
    ['200', {'Content-Type' => 'text/html'}, ['Wë súpport UTF-8!']]
end
 
Rack::Handler::WEBrick.run app
