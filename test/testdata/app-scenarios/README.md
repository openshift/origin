All files in the folder are loaded into the API during tests, so they should:
* Not create resources that duel with each other
* Not include allocated values that could flake (like service clusterIP addresses)
* Pay attention to creation order (for example, create pods first, then petsets, so the create doesn't race with the petset controller)
