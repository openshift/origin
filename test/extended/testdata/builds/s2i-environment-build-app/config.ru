require './app'

# Set default Puma thread and worker settings for containers
threads_count = ENV.fetch("PUMA_THREADS") { 5 }.to_i
workers_count = ENV.fetch("PUMA_WORKERS") { 2 }.to_i

# Configure Puma
if defined?(Puma)
  threads threads_count, threads_count
  workers workers_count if workers_count > 1

  preload_app!

  on_worker_boot do
    # Re-establish DB connections or any other resource your app uses
    puts "Puma worker booting..."
  end
end

# Default Sinatra app run
run Sinatra::Application
