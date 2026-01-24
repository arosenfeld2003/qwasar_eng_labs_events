import random
import json

# Define event types and their respective teams
teams = {
    'Security': ['brawl', 'not_on_list', 'accident'],
    'Clean_up': ['dirty_table', 'broken_itens', 'dirty_floor'],
    'Catering': ['bad_food', 'music', 'feeling_ill'],
    'Officiant': ['bride', 'groom'],
    'Waiters': ['broken_itens', 'accident', 'bad_food'] 
}

# Priority to simulation time mapping
priority_time = {
    'High': 5,      # 5 seconds in simulation
    'Medium': 10,   # 10 seconds in simulation
    'Low': 15       # 15 seconds in simulation
}

id_counter = 1

# Generate random events
def generate_random_event(id_counter):
    team = random.choice(list(teams.keys()))
    event_type = random.choice(teams[team])
    priority = random.choice(list(priority_time.keys()))
    
    descriptions = {
            "bad_food": "guest has stomach ache after eating 5 pieces of cake",
            "broken_itens": "broken glass found near the bar counter",
            "bride": "missing bride's bouquet during ceremony",
            "music": "loud music disturbing guests at table 5",
            "dirty_floor": "A guest slipped on spilled wine near the dance floor",
            "dirty_floor": "Found broken glass in the reception hall",
            "music": "Loud music disrupted the dinner service",
            "groom": "Missing groom's ring moments before the ceremony",
            "bad_food": "Guests complaining about cold food served at table 8",
            "brawl": "A brawl broke out near the bar area",
            "dirty_floor": "Someone fell due to slippery floor near the entrance",
            "bride": "The bride's bouquet went missing just before the toss",
            "dirty_floor": "Bad smell reported from the restroom area",
            "dirty_floor": "Broken chair found in the dining area",
            "not_on_list": "Not on the guest list causing confusion at the entrance",
            "bad_food": "Missing wedding cake topper moments before cutting",
            "bad_food": "Guest with a severe allergic reaction to seafood",
            "music": "Music volume too low, guests can't hear speeches",
            "dirty_floor": "Water leak spotted near the restroom area",
            "accident": "Children playing dangerously near the catering setup",
            "feeling_ill": "Guest feeling faint after dancing too long",
            "dirty_table": "Dirty dishes piling up in the kitchen area",
            "brawl": "Confusion about seating arrangements at table 5",
            "dirty_table": "Flower arrangement knocked over at the reception desk",
            "accident": "A guest's dress ripped on the dance floor",
            "accident": "Overcrowded dance floor causing discomfort",
            "music": "Speaker malfunction during the ceremony",
            "dirty_table": "Missing centerpiece on table 10",
            "broken_itens": "Broken table leg discovered in the dining area",
            "brawl": "Guests complaining of loud noise from adjacent event",
            "accident": "Injured kid after tripping on loose carpet",
            "accident": "Waiter spilled drinks on guests at table 3",
            "bad_food": "Someone feeling unwell after tasting the appetizers",
            "bad_food": "Guests frustrated with slow bar service",
            "dirty_table": "Dirty napkins left on tables after dessert service",
            "bride": "Sudden power outage during the speeches",
            "accident": "Guest missing from their assigned table",
            "bad_food": "Improperly cooked meat served to guests",
            "accident": "Sudden rain causing chaos in the outdoor reception",
            "accident": "Guest lost an earring in the restroom",
            "bad_food": "Overcooked vegetables served at table 7",
            "brawl": "Catering staff arguing loudly in the kitchen area",
            "music": "Guests complaining about poor lighting in the venue",
            "dirty_floor": "Slippery dance floor causing multiple falls",
            "music": "No-show photographer causing delay in picture taking",
            "broken_itens": "Missing gift cards from the gift table",
            "broken_itens": "Broken microphone during the toast",
            "bride": "Guest spilled red wine on their white dress",
            "accident": "Disoriented guest wandering near the exit",
            "dirty_floor": "Bad odor detected near the garbage area",
            "accident": "Missing place cards causing confusion at tables",
            "brawl": "Disorganized coat check leading to delays",
            "feeling_ill": "Guest allergic reaction to flower pollen",
            "accident": "Broken vase near the entrance causing hazard",
            "music": "Musician playing off-key during the ceremony",
            "bad_food": "Catering staff shortage causing slow food service",
            "bad_food": "Guests complaining about overly spicy food",
            "dirty_table": "Tablecloth torn during setup",
            "brawl": "Miscommunication causing late arrival of the wedding cake",
            "not_on_list": "Missing reservation for VIP guests",
            "music": "Speaker system feedback disrupting the vows",
            "feeling_ill": "Guest feeling dizzy due to strong perfume",
            "feeling_ill": "Overly bright lights causing discomfort during dinner",
            "not_on_list": "Pet found in the venue, causing allergy concerns",
            "accident": "Broken lock on restroom door",
            "accident": "Missing table number signs",
            "accident": "Guest accidentally locked themselves in the restroom",
            "brawl": "Bartender spilled cocktails on guests at the bar",
            "accident": "Catering truck blocking the entrance",
            "feeling_bad": "Overheated venue causing discomfort to guests",
            "dirty_table": "Guest spilled coffee on the rental linens",
            "bad_food": "Missing vegan options for some guests",
            "broken_itens": "Photographer's camera malfunction during family photos",
            "music": "Improperly placed speakers causing uneven sound",
            "accident": "Sudden wind gusts blowing away outdoor decorations",
            "bad_food": "Staff forgetting to refill water glasses at tables",
            "bad_food": "Guest complaining about overly salty appetizers",
            "broken_itens": "Missing wedding program pamphlets",
            "broken_itens": "Broken air conditioning unit in the dining area",
            "accident": "Guest slipped on wet floor near the restroom",
            "brawl": "Floral arrangements delivered with wilted flowers",
            "brawl": "Confusion regarding parking for elderly guests",
            "broken_itens": "Broken umbrella stand near the entrance",
            "accident": "Guests arriving late due to traffic jam",
            "broken_itens": "Missing tablecloths for outdoor seating",
            "brawl": "Waiter spilled soup on guests at table 12",
            "bad_food": "Catering staff forgot to bring out dessert plates",
            "brawl": "Sudden noise from nearby construction disrupting vows",
            "broken_itens": "Guest accidentally broke a vase on display",
            "accident": "Miscommunication led to insufficient chairs for guests",
            "dirty_table": "Missing table settings for head table",
            "bad_food": "Vendor mix-up with cake flavors",
            "feeling_bad": "Guest having difficulty breathing due to pollen allergy",
            "broken_itens": "Broken tent pole in outdoor reception area",
            "brawl": "Guest having trouble finding designated smoking area",
            "dirty_floor": "Dirty footprints noticed on the dance floor",
            "feeling_ill": "Unexpected fog rolling in during outdoor ceremony",
            "brawl": "Overcrowded shuttle bus delaying guest arrival",
            "brawl": "Guest spilled sauce on rental tuxedo",
            "not_on_list": "Missing name tags for reserved seats",
            "broken_itens": "Photographer's assistant forgot to bring extra batteries",
            "music": "Wrong song played during first dance",
            "feeling_ill": "Guest feeling nauseous after eating undercooked meat",
            "brawl": "Disorganized valet parking causing delays in car retrieval",

    }
    description = random.choice(list(descriptions.values()))
    
    # Calculate duration in simulation time
    duration_minutes = priority_time[priority]
    duration_seconds = duration_minutes * 60
    time_stamp_raw = random.randint(1, 360)
    time_stamp_hour = int(time_stamp_raw / 60)
    time_stamp_minute = (time_stamp_raw % 60) if (time_stamp_raw % 60) > 9 else f"0{(time_stamp_raw % 60)}"
    
    # Construct the event dictionary
    event = {
        'id': id_counter, 
        'event_type': event_type,
        'priority': priority,
        'description': description,
        'timestamp': f"0{time_stamp_hour}:{time_stamp_minute}" 
    }
    
    return event

total_time = 0
dataset = []
while id_counter <= 1000:
    event = generate_random_event(id_counter)
    dataset.append(event)
    id_counter += 1



#Write dataset to file
with open('dataset_5.json', 'a') as convert_file:
    convert_file.write(json.dumps(dataset))

# Print the generated dataset
for idx, event in enumerate(dataset):
    print(f"Event {idx+1}:")
    print(f"  id: {event['id']}")
    print(f"  Event Type: {event['event_type']}")
    print(f"  Priority: {event['priority']}")
    print(f"  Description: {event['description']}")
    print(f"  Timestamp: {event['timestamp']}")
    print()

