import requests

session = requests.Session()
response = session.get("https://internal.example.com", verify = False)
